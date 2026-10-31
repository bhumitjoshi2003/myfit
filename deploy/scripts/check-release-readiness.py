#!/usr/bin/env python3
"""Focused local readiness diagnosis and 90-second read-only resource check.
No login, fixtures, migrations, application changes or remote deployment.
"""
import concurrent.futures,json,os,pathlib,secrets,shlex,subprocess,time,urllib.request,urllib.error,urllib.parse
ROOT=pathlib.Path(__file__).resolve().parents[2]
os.chdir(ROOT)
def cmd(*args):
 p=subprocess.run(args,text=True,stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=60)
 if p.returncode:raise RuntimeError('Command failed: '+args[0]+' '+args[1]+'; details suppressed')
 return p.stdout.strip()
def require(condition,message):
 if not condition:raise RuntimeError(message)
def envfile(path):return {k:shlex.split(v)[0] for k,v in (l.split('=',1) for l in path.read_text().splitlines() if l.strip() and not l.lstrip().startswith('#') and '=' in l)}
http=urllib.request.build_opener(urllib.request.ProxyHandler({}))
def probe(path='/health',port=18080):
 try:
  with http.open('http://127.0.0.1:'+str(port)+path,timeout=3) as r:
   r.read();return {'status':r.status}
 except urllib.error.HTTPError as e:return {'status':e.code}
 except Exception as e:return {'error_type':type(e).__name__}
out=pathlib.Path((ROOT/'.run/amd64-release-path').read_text().strip());release=json.loads((out/'release.json').read_text())
report={'release':release['release'],'status':'started','writes_performed':False,'application_tests_repeated':False,'samples':[]}
name='myfit-readiness-'+secrets.token_hex(4);proxy=name+'-relay';inside=name+'-internal';outside=name+'-egress';created=[];networks=[]
private=ROOT/'.run'/release['release']/'readiness.env';private.parent.mkdir(parents=True,exist_ok=True)
start=time.monotonic()
def sample():
 values=list(map(int,cmd('docker','exec',name,'/app/myfit','memory-stats').split(',')))
 state=json.loads(cmd('docker','inspect',name))[0]
 s=dict(zip(['current','peak','limit','oom','oom_kill'],values));s['restarts']=state['RestartCount'];report['samples'].append(s)
 require(s['limit']==96*1024**2 and s['peak']<=72*1024**2,'96 MiB memory qualification failed: insufficient headroom')
 require(not s['oom'] and not s['oom_kill'] and not s['restarts'] and state['State']['Running'] and not state['State']['OOMKilled'],'OOM, restart or container exit detected')
 return s
try:
 e=envfile(ROOT/'deploy/production.env');staging=envfile(ROOT/'deploy/neon-staging.env');u=urllib.parse.urlsplit(e['DATABASE_URL'])
 require(e['DATABASE_URL']==staging['DATABASE_URL'] and staging['CONFIRM_DEDICATED_MYFIT']=='true','Dedicated target mismatch')
 require(u.username=='myfit_runtime' and u.path=='/neondb' and u.hostname.startswith(staging['NEON_EXPECTED_ENDPOINT']+'-pooler.') and u.hostname.endswith('.neon.tech') and urllib.parse.parse_qs(u.query).get('sslmode')==['verify-full'],'Runtime role/target/TLS mismatch')
 image=release['deployment_image_reference'];info=json.loads(cmd('docker','image','inspect',image))[0]
 require(info['Architecture']=='amd64' and info['Os']=='linux','Final image architecture mismatch')
 report['image']=image;report['image_id']=info['Id'];report['platform']='linux/amd64 under ARM emulation'
 runtime={'DATABASE_URL':e['DATABASE_URL'],'APP_ENV':'production','APP_ORIGIN':'https://myfit.edunexify.co.in','COOKIE_SECURE':'true','AUTO_MIGRATE':'false','API_ADDR':'0.0.0.0:8080','DB_MAX_CONNS':'4','GOMEMLIMIT':'64MiB','GOMAXPROCS':'1'}
 fd=os.open(private,os.O_WRONLY|os.O_CREAT|os.O_EXCL,0o600)
 with os.fdopen(fd,'w') as f:f.write(''.join(k+'='+v+'\n' for k,v in runtime.items()))
 for network,internal in [(outside,False),(inside,True)]:
  args=['docker','network','create','--label','com.myfit.validation=true']
  if internal:args+=['--internal']
  cmd(*args,network);networks.append(network)
 cmd('docker','run','-d','--name',proxy,'--label','com.myfit.validation=true','--network',outside,'-p','127.0.0.1:18082:8081','-e','NEON_HOST='+u.hostname,'--mount','type=bind,src='+str(ROOT/'deploy/scripts/neon-fault-relay.py')+',dst=/relay.py,readonly','python:3.12-alpine','python','/relay.py');created.append(proxy)
 cmd('docker','network','connect',inside,proxy)
 ip=json.loads(cmd('docker','inspect',proxy))[0]['NetworkSettings']['Networks'][inside]['IPAddress']
 # Gate the relay listener; process start alone does not establish readiness.
 relay_deadline=time.monotonic()+15
 while probe('/status',18082).get('status')!=200:
  require(time.monotonic()<relay_deadline,'Test relay not ready; API not started')
  time.sleep(.1)
 cmd('docker','run','-d','--name',name,'--label','com.docker.compose.project=myfit-memory-test','--platform','linux/amd64','--network',inside,'--add-host',u.hostname+':'+ip,'-p','127.0.0.1:18080:8080','--env-file',str(private),'--memory','96m','--memory-swap','96m','--cpus','.5','--pids-limit','64','--read-only','--cap-drop','ALL','--security-opt','no-new-privileges:true','--restart','on-failure:3',image);created.append(name)
 # Reproduce original topology; Docker healthcheck is independent of published ports.
 deadline=time.monotonic()+45
 while time.monotonic()<deadline:
  state=json.loads(cmd('docker','inspect',name))[0]
  if state['State'].get('Health',{}).get('Status')=='healthy':break
  time.sleep(1)
 require(state['State'].get('Health',{}).get('Status')=='healthy','Internal readiness failed')
 report['before']={'internal_health':'healthy','host_health':probe(),'host_livez':probe('/livez'),'relay_control':probe('/status',18082)}
 curl=subprocess.run(['curl','--noproxy','*','--silent','--output','/dev/null','--write-out','%{http_code}','--max-time','3','http://127.0.0.1:18080/health'],text=True,stdout=subprocess.PIPE,stderr=subprocess.PIPE)
 report['before']['curl_exit']=curl.returncode;report['before']['curl_status']=curl.stdout
 print('Original topology: '+json.dumps(report['before']),flush=True)
 require(report['before']['host_health'].get('status')!=200 and report['before']['host_livez'].get('status')!=200 and report['before']['relay_control'].get('status')==200 and curl.returncode!=0,'Original failure not reproduced as isolated published-port issue')
 # Single-variable fix: add a normal bridge for host-published traffic.
 # Neon hostname still points to the same opaque relay on the internal network.
 cmd('docker','network','connect',outside,name)
 report['after']={'host_health':probe(),'host_livez':probe('/livez')}
 require(report['after']['host_health'].get('status')==200 and report['after']['host_livez'].get('status')==200,'Readiness did not recover after bridge attachment')
 print('Corrected topology: '+json.dumps(report['after']),flush=True)
 report['root_cause']='API attached only to Docker internal bridge: published port inaccessible from host. Normal bridge attachment restored host health/livez without changing the API image or TLS relay.'
 sample();deadline=time.monotonic()+90
 def worker(i):
  count=0
  while time.monotonic()<deadline:
   result=probe('/livez' if i==3 else '/health')
   require(result.get('status')==200,'Workload request failed')
   count+=1;time.sleep(.25)
  return count
 with concurrent.futures.ThreadPoolExecutor(max_workers=4) as pool:
  futures=[pool.submit(worker,i) for i in range(4)]
  next_report=time.monotonic()
  while any(not f.done() for f in futures):
   s=sample()
   for f in futures:
    if f.done():f.result()
   if time.monotonic()>=next_report:
    print('Short workload: peak=%.2f MiB, OOM=%d, restarts=%d'%(s['peak']/1024**2,s['oom'],s['restarts']),flush=True);next_report=time.monotonic()+30
   time.sleep(5)
  report['requests']=sum(f.result() for f in futures)
 sample()
 report.update(status='passed',workload_seconds=90,concurrency=4,workload='Three DB-backed /health readers plus one /livez reader; 250 ms pacing per worker. Read-only representative steady DB pool/HTTP traffic, excludes auth/hash and full application paths.',peak_bytes=max(s['peak'] for s in report['samples']),oom=0,oom_kill=0,restarts=0)
except Exception as error:
 report['status']='FAILED';report['failure']=str(error) if isinstance(error,RuntimeError) else type(error).__name__+'; details suppressed'
 print(report['failure'],flush=True)
finally:
 report['elapsed_seconds']=round(time.monotonic()-start,2)
 (out/'readiness-investigation.json').write_text(json.dumps(report,indent=2)+'\n')
 for container in reversed(created):
  subprocess.run(['docker','stop','-t','20',container],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
  subprocess.run(['docker','rm',container],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
 for network in reversed(networks):subprocess.run(['docker','network','rm',network],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
 private.unlink(missing_ok=True)
print('Focused validation: '+report['status'],flush=True)
raise SystemExit(0 if report['status']=='passed' else 1)

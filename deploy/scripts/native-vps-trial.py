#!/usr/bin/env python3
"""Explicitly approved private VPS trial only; always stops only its MyFit API.
Run in the staged MyFit release directory under a detached supervisor/SSH session.
No Nginx, DNS, migrations, database writes, image build or existing-container changes.
"""
import argparse,json,pathlib,signal,subprocess,time,urllib.request,urllib.parse,shlex,platform,socket
p=argparse.ArgumentParser();p.add_argument('--edunexify-health',action='append',required=True);p.add_argument('--seconds',type=int,default=180);args=p.parse_args()
assert 30<=args.seconds<=300
http=urllib.request.build_opener(urllib.request.ProxyHandler({}))
compose=['docker','compose','--project-name','myfit-prod','--env-file','runtime.env','-f','compose.production.yml','-f','compose.trial.yml']
report={'status':'preflight','samples':[]};cid=None;start_attempted=False;start=time.monotonic();last_swap=None;swap_streak=0;slow_streak=0
class Abort(Exception):pass
def require(ok,message):
 if not ok:raise Abort(message)
def run(*cmd,timeout=15):
 result=subprocess.run(cmd,text=True,stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=timeout)
 require(result.returncode==0,'Command failed: '+cmd[0]+' '+cmd[1]+'; details withheld')
 return result.stdout.strip()
def inspect(container):return json.loads(run('docker','inspect',container))[0]
def available():
 return int(next(l.split()[1] for l in pathlib.Path('/proc/meminfo').read_text().splitlines() if l.startswith('MemAvailable:')))*1024
def swap():
 v=dict(l.split() for l in pathlib.Path('/proc/vmstat').read_text().splitlines() if l.startswith(('pswpin ','pswpout ')))
 return int(v['pswpin']),int(v['pswpout'])
def request(url):
 started=time.monotonic()
 with http.open(url,timeout=3) as response:
  require(response.status==200,'Health endpoint returned non-200');response.read(4096)
 return time.monotonic()-started
def identity(i):
 s=i['State'];return {'id':i['Id'],'started_at':s['StartedAt'],'restarts':i['RestartCount'],'health':s.get('Health',{}).get('Status'),'running':s['Running']}
def interrupted(signum,frame):raise Abort('Supervisor interrupted; stopping only MyFit')
for sig in [signal.SIGTERM,signal.SIGINT,signal.SIGHUP]:signal.signal(sig,interrupted)
try:
 require(pathlib.Path('runtime.env').stat().st_mode&0o777==0o600,'Runtime env must have mode 0600')
 text=pathlib.Path('runtime.env').read_text();require('MIGRATION_DATABASE_URL' not in text,'Migration credential must not be on VPS')
 values={k:shlex.split(v)[0] for k,v in (l.split('=',1) for l in text.splitlines() if l and not l.startswith('#') and '=' in l)}
 u=urllib.parse.urlsplit(values['DATABASE_URL'])
 require(u.username=='myfit_runtime' and u.path=='/neondb' and u.hostname.startswith('ep-square-mode-azj6avgc-pooler.') and u.hostname.endswith('.neon.tech') and urllib.parse.parse_qs(u.query).get('sslmode')==['verify-full'],'Unexpected runtime role, target or TLS')
 expected='myfit-api@sha256:a712b40445c47d466f0bb32859608478caf59d82d63b56b1ffac0506d2edbe07'
 require(values['MYFIT_IMAGE']==expected,'Unexpected image reference')
 image=json.loads(run('docker','image','inspect',expected))[0]
 require(platform.machine()=='x86_64' and image['Os']=='linux' and image['Architecture']=='amd64' and image['Config']['User']=='65532:65532','Native image/platform/user mismatch')
 require(not run('docker','network','ls','-q','--filter','name=^myfit-prod-network$'),'MyFit network already exists; review before reuse')
 with socket.socket() as port_check:port_check.bind(('127.0.0.1',18080))
 require(available()>=224*1024**2,'Preflight requires at least 224 MiB MemAvailable: 96 MiB cap plus 128 MiB host reserve')
 require(not run('docker','ps','-aq','--filter','label=com.docker.compose.project=myfit-prod'),'Existing MyFit project found; refusing to recreate it')
 existing=run('docker','ps','-q').splitlines();baseline={x:identity(inspect(x)) for x in existing}
 report['existing_containers']=baseline
 require(all(v['health'] not in ['unhealthy','starting'] for v in baseline.values()),'Existing production health is not stable')
 times=[]
 for unused in range(3):
  for url in args.edunexify_health:times.append(request(url))
  time.sleep(1)
 latency_limit=max(1.0,3*max(times));report['edunexify_latency_limit_seconds']=latency_limit
 swap_before=swap();time.sleep(5);require(swap()==swap_before,'Active host swapping during baseline; trial not started')
 run(*compose,'config','--quiet')
 # The project must be absent; this creates only MyFit and its own bridge.
 # restart=no prevents retries before monitoring can react.
 start_attempted=True
 run(*compose,'up','-d','--no-deps','--pull','never','api',timeout=45)
 cid=run(*compose,'ps','-aq','api');require(bool(cid),'MyFit container ID missing')
 i=inspect(cid);h=i['HostConfig'];e=i['Config']['Env']
 require(h['Memory']==96*1024**2 and h['MemorySwap']==96*1024**2 and h['NanoCpus']==500000000,'Unexpected resource limits')
 require(h['RestartPolicy']['Name']=='no','Trial auto-restart must be disabled')
 require(h['PortBindings']=={'8080/tcp':[{'HostIp':'127.0.0.1','HostPort':'18080'}]},'Unexpected port publication')
 require(not any(x.startswith('MIGRATION_DATABASE_URL=') for x in e),'Owner credential in API')
 report['status']='running';deadline=time.monotonic()+args.seconds;startup_deadline=time.monotonic()+45;ready=False
 while time.monotonic()<deadline:
  i=inspect(cid);s=i['State']
  require(s['Running'] and not s['OOMKilled'] and i['RestartCount']==0,'MyFit exited, OOM-killed or restarted')
  values=list(map(int,run('docker','exec',cid,'/app/myfit','memory-stats').split(',')))
  current,peak,limit,oom,killed=values
  require(limit==96*1024**2 and peak<=72*1024**2,'Native peak exceeds conservative 72 MiB headroom gate')
  require(not oom and not killed,'MyFit OOM event detected')
  mem=available();require(mem>=128*1024**2,'Host memory reserve fell below 128 MiB')
  counters=swap();active=last_swap is not None and counters!=last_swap;swap_streak=swap_streak+1 if active else 0;last_swap=counters
  require(swap_streak<2,'Sustained host swap activity over two intervals')
  for container,old in baseline.items():
   now=identity(inspect(container));require(now['running'] and now['started_at']==old['started_at'] and now['restarts']==old['restarts'],'Existing production container state changed')
   require(now['health']!='unhealthy','Existing production container is unhealthy')
  latency=max(request(url) for url in args.edunexify_health);slow_streak=slow_streak+1 if latency>latency_limit else 0
  require(slow_streak<2,'Sustained Edunexify latency regression')
  try:request('http://127.0.0.1:18080/health');ready=True
  except Exception:
   require(not ready and time.monotonic()<startup_deadline,'MyFit readiness failed after startup allowance')
  report['samples'].append({'elapsed_seconds':round(time.monotonic()-start,2),'current':current,'peak':peak,'limit':limit,'oom':oom,'oom_kill':killed,'restarts':i['RestartCount'],'host_available':mem,'pswpin':counters[0],'pswpout':counters[1],'edunexify_latency':latency,'myfit_ready':ready})
  pathlib.Path('native-trial-report.json').write_text(json.dumps(report,indent=2)+'\n')
  time.sleep(5)
 require(ready,'MyFit never became ready');report['status']='passed';report['peak_bytes']=max(x['peak'] for x in report['samples'])
except Exception as error:
 report['status']='FAILED';report['failure']=str(error) if isinstance(error,Abort) else type(error).__name__+'; details withheld'
finally:
 # Stop only the trial API, including a container created just before an up timeout.
 if cid is None and start_attempted:
  try:cid=run(*compose,'ps','-aq','api')
  except Exception:pass
 if cid:
  try:
   run('docker','stop','-t','5',cid);report['stopped']=not inspect(cid)['State']['Running'];report['final_state']=inspect(cid)['State']
  except Exception:
   report['status']='FAILED';report['stop_error']='Could not confirm MyFit stopped; operator must stop only its recorded container ID'
 pathlib.Path('native-trial-report.json').write_text(json.dumps(report,indent=2)+'\n')
 print(json.dumps({k:v for k,v in report.items() if k not in ['samples','existing_containers','final_state']},indent=2))

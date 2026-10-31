#!/usr/bin/env python3
"""Isolated local amd64 release qualification. Retains identifiable Neon fixtures."""
import shlex,tarfile,hashlib
import concurrent.futures,datetime,http.cookies,json,os,pathlib,secrets,subprocess,time,urllib.request,urllib.error,urllib.parse
ROOT=pathlib.Path(__file__).resolve().parents[2]
os.chdir(ROOT)
def command(*args):
 p=subprocess.run(args,text=True,stdout=subprocess.PIPE,stderr=subprocess.PIPE)
 if p.returncode:raise RuntimeError('Local command failed: '+args[0]+' '+args[1]+' (details suppressed)')
 return p.stdout.strip()
def envfile(path):
 return {k:shlex.split(v)[0] for k,v in (line.strip().split('=',1) for line in path.read_text().splitlines() if line.strip() and not line.lstrip().startswith('#') and '=' in line)}
def check(ok,message):
 if not ok:raise RuntimeError(message)
class Client:
 def __init__(self):self.cookie='';self.csrf=''
 def req(self,path,method='GET',body=None,status=200):
  headers={'Origin':'https://myfit.edunexify.co.in','Content-Type':'application/json','Cookie':self.cookie,'X-CSRF-Token':self.csrf}
  r=urllib.request.Request('http://127.0.0.1:18080'+path,data=None if body is None else json.dumps(body).encode(),method=method,headers=headers)
  try:response=urllib.request.urlopen(r,timeout=15)
  except urllib.error.HTTPError as e:response=e
  with response:
   check(response.status==status,f'{method} {path} status {response.status}, expected {status}; response suppressed')
   cookie=response.headers.get('Set-Cookie')
   if cookie:
    jar=http.cookies.SimpleCookie();jar.load(cookie);c=jar['__Host-myfit_session']
    check(c['secure'] and c['httponly'] and not c['domain'] and c['path']=='/','Cookie security failed')
    self.cookie=c.key+'='+c.value
   raw=response.read()
   return json.loads(raw) if raw else None
out=pathlib.Path((ROOT/'.run/amd64-release-path').read_text().strip())
release=json.loads((out/'release.json').read_text());run=ROOT/'.run'/release['release'];run.mkdir(mode=0o700,exist_ok=True)
report={'release':release['release'],'status':'started','platform':'linux/amd64 under local ARM emulation','memory_limit_bytes':96*1024**2,'fixtures_retained':True}
api='myfit-soak-'+secrets.token_hex(4);proxy=api+'-relay';inside=api+'-internal';outside=api+'-egress'
created=[];networks=[];samples=[];requests=0
start=time.monotonic()
def save(): (out/'container-validation.json').write_text(json.dumps(report,indent=2)+'\n')
def sample():
 values=command('docker','exec',api,'/app/myfit','memory-stats').split(',')
 current,peak,limit,oom,killed=map(int,values)
 samples.append({'elapsed_seconds':round(time.monotonic()-start,2),'current':current,'peak':peak,'limit':limit,'oom':oom,'oom_kill':killed})
 state=json.loads(command('docker','inspect',api))[0]
 check(limit==96*1024**2 and peak<=72*1024**2,'RESOURCE CHECK FAILED: 96 MiB limit lacks required 25% peak headroom')
 check(not oom and not killed and state['State']['Running'] and not state['State']['OOMKilled'] and state['RestartCount']==0,'RESOURCE CHECK FAILED: OOM, exit or restart')
 return peak
def control(path):
 with urllib.request.urlopen('http://127.0.0.1:18082/'+path,timeout=5) as r:return json.load(r)
def wait_health():
 deadline=time.monotonic()+45
 while time.monotonic()<deadline:
  try:Client().req('/health');return
  except (RuntimeError,urllib.error.URLError):time.sleep(1)
 raise RuntimeError('Neon health did not recover within 45 seconds')
def workload(clients,seconds):
 global requests
 deadline=time.monotonic()+seconds
 def worker(c):
  count=0
  while time.monotonic()<deadline:
   c.req('/api/foods?q=banana');c.req('/api/diary?date='+day);count+=2;time.sleep(.5)
  return count
 with concurrent.futures.ThreadPoolExecutor(max_workers=4) as executor:
  futures=[executor.submit(worker,clients[i%2]) for i in range(4)]
  next_progress=time.monotonic()
  while any(not f.done() for f in futures):
   peak=sample()
   for f in futures:
    if f.done():f.result()
   if time.monotonic()>=next_progress:
    print('Soak running: elapsed=%ds peak=%.2f MiB'%(time.monotonic()-start,peak/1024**2),flush=True);next_progress=time.monotonic()+30
   time.sleep(5)
  requests+=sum(f.result() for f in futures)
try:
 e=envfile(ROOT/'deploy/production.env');staging=envfile(ROOT/'deploy/neon-staging.env')
 u=urllib.parse.urlsplit(e['DATABASE_URL']);q=urllib.parse.parse_qs(u.query)
 check(e['DATABASE_URL']==staging['DATABASE_URL'] and staging['CONFIRM_DEDICATED_MYFIT']=='true','Target confirmation mismatch')
 check(u.username=='myfit_runtime' and u.path=='/neondb' and u.hostname.startswith(staging['NEON_EXPECTED_ENDPOINT']+'-pooler.') and u.hostname.endswith('.neon.tech') and q.get('sslmode')==['verify-full'],'SECURITY CHECK FAILED: runtime target/role/TLS mismatch')
 info=json.loads(command('docker','image','inspect',release['tag']))[0]
 check(info['Architecture']=='amd64' and info['Os']=='linux' and info['Config']['User']=='65532:65532','COMPATIBILITY/SECURITY CHECK FAILED: image')
 with tarfile.open(out/'myfit-api.tar') as archive:
  saved=json.load(archive.extractfile('manifest.json'))[0]
  release['image_config_digest']='sha256:'+hashlib.sha256(archive.extractfile(saved['Config']).read()).hexdigest()
 release['local_image_id']=info['Id'];release['deployment_image_reference']=release['tag'].split(':')[0]+'@'+release['manifest_digest']
 (out/'release.json').write_text(json.dumps(release,indent=2)+'\n')
 runtime={'DATABASE_URL':e['DATABASE_URL'],'APP_ENV':'production','APP_ORIGIN':'https://myfit.edunexify.co.in','COOKIE_SECURE':'true','AUTO_MIGRATE':'false','API_ADDR':'0.0.0.0:8080','DB_MAX_CONNS':'4','GOMEMLIMIT':'64MiB','GOMAXPROCS':'1'}
 private=run/'api.env';private.write_text(''.join(k+'='+v+'\n' for k,v in runtime.items()));private.chmod(0o600)
 # Only this new container gets an overridden hostname; host and public DNS are untouched.
 for name,internal in [(outside,False),(inside,True)]:
  args=['docker','network','create','--label','com.myfit.validation=true']
  if internal:args+=['--internal']
  command(*args,name);networks.append(name)
 command('docker','run','-d','--name',proxy,'--label','com.myfit.validation=true','--network',outside,'-p','127.0.0.1:18082:8081','-e','NEON_HOST='+u.hostname,'--mount','type=bind,src='+str(ROOT/'deploy/scripts/neon-fault-relay.py')+',dst=/relay.py,readonly','python:3.12-alpine','python','/relay.py');created.append(proxy)
 command('docker','network','connect',inside,proxy)
 ip=json.loads(command('docker','inspect',proxy))[0]['NetworkSettings']['Networks'][inside]['IPAddress']
 # Wait for the test relay listener before the API's first database connection.
 relay_deadline=time.monotonic()+15
 while True:
  try:
   check(not control('status')['blocked'],'Test relay unexpectedly blocked');break
  except urllib.error.URLError:
   if time.monotonic()>=relay_deadline:raise RuntimeError('Test relay did not become ready; API not started')
   time.sleep(.1)
 command('docker','run','-d','--name',api,'--label','com.docker.compose.project=myfit-memory-test','--platform','linux/amd64','--network',inside,'--add-host',u.hostname+':'+ip,'-p','127.0.0.1:18080:8080','--env-file',str(private),'--memory','96m','--memory-swap','96m','--cpus','.5','--pids-limit','64','--read-only','--cap-drop','ALL','--security-opt','no-new-privileges:true','--restart','on-failure:3',release['deployment_image_reference']);created.append(api)
 # Docker does not publish host ports for this internal-only topology.
 # Keep the same Neon relay override; add a normal bridge for loopback publishing.
 command('docker','network','connect',outside,api)
 # Gate before any API writes. If resource check fails, stop here.
 wait_health();sample()
 state=json.loads(command('docker','inspect',api))[0]
 check(not any(x.startswith('MIGRATION_DATABASE_URL=') for x in state['Config']['Env']),'SECURITY CHECK FAILED: owner credential in container')
 report['runtime_role']='myfit_runtime';report['tls']='verify-full; opaque relay does not terminate TLS';report['owner_credential_in_container']=False
 print('Container ready under 96 MiB; starting identifiable fixture checks',flush=True)
 # Guard ordinary session housekeeping before registration; no existing rows deleted.
 pg_env=os.environ.copy();pg_env['PGDATABASE']=e['DATABASE_URL']
 guard=subprocess.run(['psql','-X','-A','-t','-v','ON_ERROR_STOP=1','-c',"SELECT count(*) FROM public.sessions WHERE expires_at < now()+interval '20 minutes'"],env=pg_env,text=True,stdout=subprocess.PIPE,stderr=subprocess.PIPE)
 check(guard.returncode==0 and guard.stdout.strip()=='0','SECURITY CHECK FAILED: session preservation guard; no fixture writes performed')
 password=secrets.token_hex(24);stamp=release['release']+'-'+secrets.token_hex(3)
 clients=[Client(),Client()];emails=['myfit-amd64-soak-'+stamp+'-'+x+'@example.test' for x in ['a','b']]
 report['test_emails']=emails;save()
 for c,email in zip(clients,emails):
  v=c.req('/api/register','POST',{'name':'MYFIT AMD64 SOAK '+stamp,'email':email,'password':password,'timezone':'Asia/Kolkata'});c.csrf=v['csrf']
 day=datetime.datetime.now(datetime.timezone.utc).strftime('%Y-%m-%d')
 a,b=clients
 foods=a.req('/api/foods?q=Banana%2C%20raw');food=foods[0];serving=next(s for s in food['servings'] if s['grams']==118)
 entry=a.req('/api/logs','POST',{'food_id':food['id'],'serving_id':serving['id'],'quantity':2,'day':day,'meal':'breakfast'})
 check(abs(entry['nutrition']['calories']-210.04)<.000001,'Nutrition check failed')
 entry=a.req('/api/logs/'+str(entry['id']),'PUT',{'quantity':1,'day':day,'meal':'breakfast','version':entry['version']})
 check(abs(entry['nutrition']['calories']-105.02)<.000001,'Editing check failed')
 a.req('/api/weights','POST',{'day':day,'kg':54.5})
 check(not b.req('/api/weights') and not b.req('/api/diary?date='+day)['logs'],'Isolation failed')
 old=a.csrf;a.csrf='invalid';a.req('/api/weights','POST',{'day':day,'kg':55},403);a.csrf=old
 fresh=Client();v=fresh.req('/api/login','POST',{'email':emails[0],'password':password});fresh.csrf=v['csrf']
 check(len(fresh.req('/api/diary?date='+day)['logs'])==1,'Login persistence failed')
 workload(clients,300)
 control('interrupt');t=time.monotonic();Client().req('/health',status=503);Client().req('/livez');time.sleep(max(0,20-(time.monotonic()-t)));sample()
 control('restore');t=time.monotonic();wait_health();report['recovery_seconds']=round(time.monotonic()-t,3)
 check(len(a.req('/api/diary?date='+day)['logs'])==1 and len(a.req('/api/weights'))==1,'Recovery persistence failed')
 a.req('/api/weights','POST',{'day':day,'kg':54.6})
 workload(clients,300);sample()
 logs=command('docker','logs',api)
 check(e['DATABASE_URL'] not in logs and urllib.parse.unquote(u.password) not in logs,'SECURITY CHECK FAILED: credential in container logs')
 report.update(status='passed',requests=requests,peak_bytes=max(s['peak'] for s in samples),oom=0,oom_kill=0,restarts=0,interruption_seconds=20,checks=['registration','login','nutrition','food logging/editing','weight tracking','isolation','CSRF','persistence','health 503/livez 200 during interruption','in-process reconnection','96 MiB limit with >=25% peak headroom'])
except Exception as error:
 report['status']='FAILED';report['failure']=str(error) if isinstance(error,RuntimeError) else 'Validation error; details suppressed to protect credentials'
 print(report['failure'],flush=True)
finally:
 report['elapsed_seconds']=round(time.monotonic()-start,2);report['samples']=samples;save()
 for name in reversed(created):
  subprocess.run(['docker','stop','-t','20',name],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
  subprocess.run(['docker','rm',name],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
 for name in reversed(networks):subprocess.run(['docker','network','rm',name],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
 if 'private' in locals():private.unlink(missing_ok=True)
print('Validation status: '+report['status'],flush=True)
raise SystemExit(0 if report['status']=='passed' else 1)

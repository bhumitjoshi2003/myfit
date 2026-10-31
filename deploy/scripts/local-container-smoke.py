#!/usr/bin/env python3
"""LOCAL ONLY: disposable schema in the existing myfit-postgres dev container.
Never accepts a remote database URL. Does not touch existing application records.
"""
import concurrent.futures, datetime, json, secrets, subprocess, time, urllib.request, urllib.error, argparse, os, pathlib
from http.cookies import SimpleCookie

def command(*args, capture=True):
    return subprocess.check_output(args, text=True).strip() if capture else subprocess.check_call(args)
def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--nginx-check',action='store_true',help='Use prepared .run/nginx-check test files for local HTTPS routing/auth verification')
    args=parser.parse_args()
    schema = 'myfit_container_' + secrets.token_hex(6)
    name = 'myfit-memory-test-' + secrets.token_hex(4)
    origin = 'https://myfit.edunexify.co.in'+(':18443' if args.nginx_check else '')
    base = 'http://127.0.0.1:18081'
    def sql(statement):
        return command('docker','exec','myfit-postgres','psql','-U','myfit','-d','myfit','-v','ON_ERROR_STOP=1','-Atc',statement)
    assert sql('SELECT current_database()') == 'myfit'
    sql('CREATE SCHEMA '+schema)
    cid = None
    nginx_id = None
    try:
        cid = command('docker','run','-d','--name',name,'--label','com.docker.compose.project=myfit-memory-test',
          '--memory=96m','--memory-swap=96m','--cpus=0.50','--pids-limit=64','--read-only','--cap-drop=ALL',
          '--security-opt=no-new-privileges:true','-p','127.0.0.1:18081:8080',
          '-e','APP_ENV=development','-e','COOKIE_SECURE=true','-e','APP_ORIGIN='+origin,
          '-e','GOMEMLIMIT=64MiB','-e','GOMAXPROCS=1','-e','DB_MAX_CONNS=4',
          '-e','DATABASE_URL=postgres://myfit:myfit_local_dev@host.docker.internal:5433/myfit?sslmode=disable&search_path='+schema,
          'myfit-api:production-check')
        def request(path, method='GET', data=None, cookie='', csrf=''):
            headers={'Origin':origin,'Content-Type':'application/json'}
            if cookie: headers['Cookie']=cookie
            if csrf: headers['X-CSRF-Token']=csrf
            req=urllib.request.Request(base+path, data=None if data is None else json.dumps(data).encode(),method=method,headers=headers)
            with urllib.request.urlopen(req,timeout=15) as response:
                return json.load(response), response.headers
        for _ in range(60):
            try: request('/health'); break
            except (OSError,urllib.error.URLError): time.sleep(1)
        else: raise RuntimeError('local test container did not become ready')
        samples=[command('docker','exec',cid,'/app/myfit','memory-stats')]
        for index in range(4):
            email=f'memory-{index}@example.test';password='memory-test-password-123'
            data,hs=request('/api/register','POST',{'name':'Memory test','email':email,'password':password,'timezone':'Asia/Kolkata'})
            jar=SimpleCookie();jar.load(hs['Set-Cookie']);c=jar['__Host-myfit_session']
            assert c['secure'] and c['httponly'] and c['path']=='/' and not c['domain']
            cookie=c.key+'='+c.value;csrf=data['csrf']
            profile={'age':24,'sex':'male','height':167.64,'height_unit':'cm','weight':54.5,'weight_unit':'kg','units':'metric','timezone':'Asia/Kolkata','lifestyle':'sedentary','sessions':3.5,'minutes':60,'intensity':'moderate','training':'resistance','objective':'muscle','diet':'vegetarian','health_context':'healthy'}
            state,_=request('/api/onboarding/profile','PUT',{'version':0,'profile':profile},cookie,csrf)
            request('/api/onboarding/goals','POST',{'version':state['user']['onboarding_version'],'recommendation_id':state['recommendation']['id'],'mode':'recommendation'},cookie,csrf)
            foods,_=request('/api/foods?q=banana',cookie=cookie);food=foods[0];serving=next(s for s in food['servings'] if s['grams']==118)
            entry,_=request('/api/logs','POST',{'food_id':food['id'],'serving_id':serving['id'],'quantity':2,'day':datetime.date.today().isoformat(),'meal':'breakfast'},cookie,csrf)
            assert abs(entry['nutrition']['calories']-210.04)<1e-6
            request('/api/weights','POST',{'day':datetime.date.today().isoformat(),'kg':54.5},cookie,csrf)
            request('/api/logout','POST',{},cookie,csrf)
            data,hs=request('/api/login','POST',{'email':email,'password':password});jar.load(hs['Set-Cookie']);c=jar['__Host-myfit_session'];cookie=c.key+'='+c.value;csrf=data['csrf']
            samples.append(command('docker','exec',cid,'/app/myfit','memory-stats'))
        if args.nginx_check:
            root=pathlib.Path(__file__).resolve().parents[2]
            nginx_id=command('docker','run','-d','--name',name+'-nginx','--memory=32m','--cpus=0.25',
                '-p','127.0.0.1:18443:443',
                '-v',str(root/'deploy/nginx/myfit.edunexify.co.in.conf')+':/etc/nginx/conf.d/default.conf:ro',
                '-v',str(root/'.run/nginx-check/snippets')+':/etc/nginx/snippets:ro',
                '-v',str(root/'.run/nginx-check/certs')+':/etc/letsencrypt/live/edunexify-wildcard:ro',
                '-v',str(root/'deploy/artifacts/site')+':/var/www/myfit/current:ro','nginx:stable-alpine')
            time.sleep(2)
            env={**os.environ,'SSL_CERT_FILE':str(root/'.run/nginx-check/certs/fullchain.pem'),'MYFIT_VERIFY_EMAIL':email,'MYFIT_VERIFY_PASSWORD':password}
            subprocess.run(['python3',str(root/'deploy/scripts/verify.py'),'--url',origin,'--connect-ip','127.0.0.1','--ca-file',str(root/'.run/nginx-check/certs/fullchain.pem'),'--auth'],env=env,check=True)
        # Two simultaneous users/read requests for 30 seconds, bounded to 10 req/s.
        def reads():
            until=time.monotonic()+30;count=0
            while time.monotonic()<until:
                request('/api/diary',cookie=cookie);request('/api/foods?q=rice',cookie=cookie);count+=2;time.sleep(.4)
            return count
        with concurrent.futures.ThreadPoolExecutor(max_workers=2) as pool:
            jobs=[pool.submit(reads) for _ in range(2)]
            while not all(j.done() for j in jobs):
                samples.append(command('docker','exec',cid,'/app/myfit','memory-stats'));time.sleep(5)
            total=sum(j.result() for j in jobs)
        samples.append(command('docker','exec',cid,'/app/myfit','memory-stats'))
        state=json.loads(command('docker','inspect',cid))[0]
        peak=max(int(s.split(',')[1]) for s in samples if s.split(',')[1].isdigit())
        events=samples[-1].split(',')
        assert state['State']['Running'] and not state['State']['OOMKilled'] and events[3:]==['0','0']
        command('docker','stop','-t','20',cid)
        exit_code=int(command('docker','inspect','--format','{{.State.ExitCode}}',cid))
        assert exit_code==0, 'API did not stop cleanly'
        print(json.dumps({'graceful_stop_exit_code':exit_code,'image':'myfit-api:production-check','architecture':command('docker','image','inspect','--format','{{.Architecture}}','myfit-api:production-check'),'image_bytes':int(command('docker','image','inspect','--format','{{.Size}}','myfit-api:production-check')),'limit_bytes':int(events[2]),'peak_cgroup_bytes':peak,'peak_mib':round(peak/1024**2,2),'oom':0,'oom_kill':0,'read_requests':total,'concurrency':2,'read_duration_seconds':30,'registrations':4,'logins':4,'tls_to_database':False,'vps_validated':False},indent=2))
    finally:
        if nginx_id: command('docker','rm','-f',nginx_id)
        if cid: command('docker','rm','-f',cid)
        sql('DROP SCHEMA '+schema+' CASCADE')
if __name__=='__main__':main()

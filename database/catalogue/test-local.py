#!/usr/bin/env python3
"""Run catalogue integration only in a freshly created disposable LOCAL database."""
import json,os,pathlib,secrets,subprocess
ROOT=pathlib.Path(__file__).resolve().parents[2]
def run(*args,**kwargs):return subprocess.check_output(args,text=True,**kwargs).strip()
context=json.loads(run('docker','context','inspect'))[0]
assert context['Endpoints']['docker']['Host'].startswith('unix://'),'Remote Docker refused'
info=json.loads(run('docker','inspect','myfit-postgres'))[0]
assert info['Config']['Image']=='postgres:17-alpine' and info['State']['Running'],'Expected local dev PostgreSQL only'
name='myfit_catalogue_test_'+secrets.token_hex(6)
run('docker','exec','myfit-postgres','psql','-X','-U','myfit','-d','postgres','-v','ON_ERROR_STOP=1','-c','CREATE DATABASE '+name)
try:
 env=os.environ.copy()
 for key in ['DATABASE_URL','MIGRATION_DATABASE_URL','TEST_DATABASE_URL']:env.pop(key,None)
 env['TEST_CATALOGUE_DATABASE_URL']='postgres://myfit:myfit_local_dev@127.0.0.1:5433/'+name+'?sslmode=disable'
 subprocess.run(['go','test','./internal/store','-run','^TestCatalogueMigration$','-count=1','-v'],cwd=ROOT/'backend',env=env,check=True)
finally:
 # Only the unpredictable database created above, never the user's myfit database.
 run('docker','exec','myfit-postgres','psql','-X','-U','myfit','-d','postgres','-v','ON_ERROR_STOP=1','-c','DROP DATABASE '+name)

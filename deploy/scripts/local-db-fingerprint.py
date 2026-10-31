#!/usr/bin/env python3
"""Read-only fingerprint of the fixed, existing local MyFit PostgreSQL container."""
import argparse, hashlib, json, pathlib, subprocess, sys

def run(*args):
    return subprocess.check_output(args, text=True, stderr=subprocess.DEVNULL).strip()
def main():
    p=argparse.ArgumentParser(description=__doc__)
    p.add_argument('--output',required=True)
    p.add_argument('--compare',help='Existing private baseline report; never restored or written to the DB')
    a=p.parse_args()
    endpoint=run('docker','context','inspect','--format','{{.Endpoints.docker.Host}}')
    if not endpoint.startswith('unix://'):raise RuntimeError('Expected a local Unix-socket Docker context; no database read performed')
    base=['docker','exec','-e','PGOPTIONS=-c default_transaction_read_only=on','myfit-postgres','psql','-X','-U','myfit','-d','myfit','-At','-v','ON_ERROR_STOP=1','-c']
    def sql(query):return run(*(base+[query]))
    if sql('SELECT current_database()')!='myfit':raise RuntimeError('Unexpected local database')
    data={}
    for name in sql("SELECT tablename FROM pg_tables WHERE schemaname='public' ORDER BY tablename").splitlines():
        quoted='"'+name.replace('"','""')+'"'
        data[name]=sql('SELECT count(*)::text||\':\'||coalesce(md5(string_agg(h,\'\' ORDER BY h)),md5(\'\')) FROM (SELECT md5(row_to_json(t)::text) h FROM public.'+quoted+' t) rows')
    for name in sql("SELECT sequencename FROM pg_sequences WHERE schemaname='public' ORDER BY sequencename").splitlines():
        data['sequence:'+name]=sql('SELECT last_value::text||\':\'||is_called::text FROM public."'+name.replace('"','""')+'"')
    schema=subprocess.check_output(['docker','exec','myfit-postgres','pg_dump','--schema-only','--no-owner','--no-privileges','-U','myfit','-d','myfit'],stderr=subprocess.DEVNULL)
    schema=b'\n'.join(line for line in schema.splitlines() if not line.startswith((b'\\restrict ',b'\\unrestrict ')))
    result={'database':'myfit','tables_and_sequences':data,'schema_sha256':hashlib.sha256(schema).hexdigest()}
    out=pathlib.Path(a.output);out.parent.mkdir(parents=True,exist_ok=True,mode=0o700);out.write_text(json.dumps(result,sort_keys=True,indent=2));out.chmod(0o600)
    print('Read-only local database fingerprints saved; no row values or credentials printed.')
    if a.compare:
        if json.loads(pathlib.Path(a.compare).read_text())!=result:raise RuntimeError('Local fingerprints differ; investigate concurrent local activity without modifying any data')
        print('PASS: local PostgreSQL schema, table contents and sequence values match the baseline.')
if __name__=='__main__':
    try:main()
    except Exception:
        print('FAIL: local fingerprint operation failed or comparison differed; no database writes were requested.',file=sys.stderr);sys.exit(1)

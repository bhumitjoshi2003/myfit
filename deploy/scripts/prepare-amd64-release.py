#!/usr/bin/env python3
"""Local build/export only. No registry push, remote commands, or deployment."""
import datetime,hashlib,json,os,pathlib,subprocess,tarfile
ROOT=pathlib.Path(__file__).resolve().parents[2]
def cmd(*args):return subprocess.check_output(args,text=True).strip()
def main():
 os.chdir(ROOT)
 paths=[ROOT/'Dockerfile',ROOT/'.dockerignore']
 for directory in ['backend','database/migrations','frontend/src']:
  paths.extend(p for p in (ROOT/directory).rglob('*') if p.is_file() and (p.suffix in ('.go','.sql','.tsx','.ts','.css') or p.name in ('go.mod','go.sum')))
 paths.extend(ROOT/'frontend'/name for name in ['package.json','package-lock.json','index.html','vite.config.ts','tsconfig.json'])
 inputs={str(p.relative_to(ROOT)):hashlib.sha256(p.read_bytes()).hexdigest() for p in sorted(set(paths))}
 digest=hashlib.sha256(json.dumps(inputs,sort_keys=True,separators=(',',':')).encode()).hexdigest()
 release=datetime.datetime.now(datetime.timezone.utc).strftime('%Y%m%dT%H%M%SZ')+'-amd64-'+digest[:12]
 out=ROOT/'deploy/artifacts'/release;out.mkdir(parents=True,exist_ok=False)
 image='myfit-api:'+release
 (out/'source-manifest.json').write_text(json.dumps({'source_sha256':digest,'inputs':inputs},indent=2)+'\n')
 subprocess.run(['docker','buildx','build','--platform','linux/amd64','--target','runtime','--provenance=false','--metadata-file',str(out/'api-build-metadata.json'),'--build-arg','RELEASE_ID='+release,'--build-arg','SOURCE_DIGEST='+digest,'--load','-t',image,'.'],check=True)
 info=json.loads(cmd('docker','image','inspect',image))[0]
 if info['Architecture']!='amd64' or info['Os']!='linux':raise RuntimeError('COMPATIBILITY CHECK FAILED: final image is not linux/amd64')
 if info['Config']['User']!='65532:65532':raise RuntimeError('SECURITY CHECK FAILED: unexpected runtime user')
 subprocess.run(['docker','buildx','build','--platform','linux/amd64','--target','frontend-assets','--output','type=local,dest='+str(out/'site'),'.'],check=True)
 subprocess.run(['docker','save','--output',str(out/'myfit-api.tar'),image],check=True)
 with tarfile.open(out/'frontend.tar.gz','w:gz') as archive:
  for p in sorted((out/'site').iterdir()):archive.add(p,arcname=p.name)
 (out/'assets.sha256').write_text(''.join(hashlib.sha256(p.read_bytes()).hexdigest()+'  '+str(p.relative_to(out/'site'))+'\n' for p in sorted((out/'site').rglob('*')) if p.is_file()))
 metadata=json.loads((out/'api-build-metadata.json').read_text())
 with tarfile.open(out/'myfit-api.tar') as archive:
  saved=json.load(archive.extractfile('manifest.json'))[0]
  config_digest='sha256:'+hashlib.sha256(archive.extractfile(saved['Config']).read()).hexdigest()
 manifest={'release':release,'tag':image,'platform':'linux/amd64','source_sha256':digest,'manifest_digest':metadata['containerimage.digest'],'image_config_digest':config_digest,'local_image_id':info['Id'],'repo_digests':info.get('RepoDigests',[]),'image_bytes':info['Size'],'archive_sha256':hashlib.sha256((out/'myfit-api.tar').read_bytes()).hexdigest(),'frontend_archive_sha256':hashlib.sha256((out/'frontend.tar.gz').read_bytes()).hexdigest(),'deployment_image_reference':'myfit-api@'+metadata['containerimage.digest'],'transport':'offline docker-save archive; not registry-published','validation_status':'pending; do not deploy'}
 (out/'release.json').write_text(json.dumps(manifest,indent=2)+'\n')
 pointer=ROOT/'.run/amd64-release-path';pointer.parent.mkdir(exist_ok=True);pointer.write_text(str(out)+'\n')
 print(json.dumps(manifest,indent=2),flush=True)
if __name__=='__main__':main()

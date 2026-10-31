"""Test-only opaque TCP forwarding; preserves PostgreSQL end-to-end TLS."""
import asyncio,json,os
blocked=False
connections=set()
async def forward(reader,writer):
 try:
  while data:=await reader.read(65536):
   writer.write(data);await writer.drain()
 finally:writer.close()
async def relay(reader,writer):
 if blocked:writer.close();return
 try:
  remote,out=await asyncio.open_connection(os.environ['NEON_HOST'],5432)
  connections.update((writer,out))
  await asyncio.gather(forward(reader,out),forward(remote,writer))
 except Exception:writer.close()
 finally:
  connections.discard(writer)
  if 'out' in locals():connections.discard(out);out.close()
async def control(reader,writer):
 global blocked
 line=await reader.readline();path=line.decode().split(' ')[1]
 if path=='/interrupt':
  blocked=True
  for c in list(connections):c.close()
 elif path=='/restore':blocked=False
 body=json.dumps({'blocked':blocked,'connections':len(connections)}).encode()
 writer.write(b'HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nConnection: close\r\nContent-Length: '+str(len(body)).encode()+b'\r\n\r\n'+body)
 await writer.drain();writer.close()
async def main():
 a=await asyncio.start_server(relay,'0.0.0.0',5432)
 b=await asyncio.start_server(control,'0.0.0.0',8081)
 async with a,b:await asyncio.gather(a.serve_forever(),b.serve_forever())
asyncio.run(main())

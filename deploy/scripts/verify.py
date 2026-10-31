#!/usr/bin/env python3
"""Read-only HTTPS checks by default. Optional session-only login/logout via env."""
import argparse, http.client, json, os, socket, ssl, sys
from http.cookies import SimpleCookie
from urllib.parse import urlsplit


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--url', default='https://myfit.edunexify.co.in')
    p.add_argument('--connect-ip', help='Connect to this IP with the URL hostname as TLS SNI/Host; verifies cert normally')
    p.add_argument('--ca-file', help='Optional explicit trusted CA bundle, for private-CA staging; never disables hostname/certificate validation')
    p.add_argument('--auth', action='store_true', help='Use MYFIT_VERIFY_EMAIL/PASSWORD; creates/revokes one session, never modifies diary/goals')
    args = p.parse_args()
    u = urlsplit(args.url)
    if u.scheme != 'https' or u.path not in ('', '/') or u.query or u.fragment or u.username:
        p.error('Use an HTTPS origin without a path or credentials')
    origin = 'https://' + u.netloc
    tls_context = ssl.create_default_context(cafile=args.ca_file)
    def request(path, method='GET', data=None, headers=None):
        conn = http.client.HTTPSConnection(u.hostname, u.port or 443, timeout=15, context=tls_context)
        if args.connect_ip:
            raw = socket.create_connection((args.connect_ip, u.port or 443), timeout=15)
            conn.sock = tls_context.wrap_socket(raw, server_hostname=u.hostname)
        h = {'Host': u.netloc, **(headers or {})}
        if data is not None:
            h['Content-Type'] = 'application/json'
        conn.request(method, path, body=None if data is None else json.dumps(data), headers=h)
        r = conn.getresponse(); body = r.read(2_000_000); result = (r.status, dict(r.getheaders()), body); conn.close(); return result
    def check(ok, message):
        if not ok:
            raise RuntimeError(message)
        print('PASS:', message)
    for path in ('/health', '/livez'):
        status, _, body = request(path)
        check(status == 200 and json.loads(body).get('status') == 'UP', path + ' healthy')
    status, headers, body = request('/')
    check(status == 200 and b'<div id="root">' in body, 'React static app served')
    for key in ('Strict-Transport-Security', 'Content-Security-Policy', 'X-Content-Type-Options', 'X-Frame-Options', 'Referrer-Policy'):
        check(bool(next((v for k,v in headers.items() if k.lower() == key.lower()), None)), key + ' present')
    check(request('/api/me')[0] == 401, 'private API rejects anonymous requests')
    check(request('/api/login', 'POST', {}, {'Origin':'https://untrusted.invalid'})[0] == 403, 'untrusted origin rejected')
    check(request('/assets/does-not-exist.js')[0] == 404, 'missing asset is not SPA HTML')
    if args.auth:
        email, password = os.getenv('MYFIT_VERIFY_EMAIL'), os.getenv('MYFIT_VERIFY_PASSWORD')
        if not email or not password:
            raise RuntimeError('set MYFIT_VERIFY_EMAIL and MYFIT_VERIFY_PASSWORD without putting credentials in arguments')
        status, hs, body = request('/api/login', 'POST', {'email':email,'password':password}, {'Origin':origin})
        check(status == 200, 'verification account login succeeds')
        csrf = json.loads(body)['csrf']
        cookie_header = next((v for k,v in hs.items() if k.lower() == 'set-cookie'), '')
        cookies = SimpleCookie(); cookies.load(cookie_header)
        check(set(cookies.keys()) == {'__Host-myfit_session'}, 'only isolated MyFit session cookie set')
        c = cookies['__Host-myfit_session']
        check(bool(c['secure']) and bool(c['httponly']) and c['samesite'].lower() == 'lax' and c['path'] == '/' and not c['domain'], 'secure host-only cookie attributes')
        auth = {'Origin':origin, 'Cookie':c.key+'='+c.value}
        try:
            check(request('/api/me', headers=auth)[0] == 200, 'session persists')
            check(request('/api/logout', 'POST', {}, auth)[0] == 403, 'CSRF token required')
        finally:
            check(request('/api/logout', 'POST', {}, {**auth,'X-CSRF-Token':csrf})[0] == 200, 'verification session logged out')
        check(request('/api/me', headers=auth)[0] == 401, 'session revoked')
    print('Verification complete. Also check Edunexify health/error rates against the pre-change baseline.')

if __name__ == '__main__':
    try: main()
    except Exception as exc:
        # Never echo responses or request bodies that could contain session credentials.
        print('FAIL:', str(exc), file=sys.stderr); sys.exit(1)

#!/usr/bin/env python3
"""Статический dev-сервер фронтенда с отключённым кэшем.

Babel компилирует .jsx в браузере на лету, поэтому кэш браузера мешает
видеть свежие правки. Этот сервер отдаёт всё с Cache-Control: no-store.
Запуск:  python3 serve.py   (порт можно задать через переменную PORT).
"""
import http.server
import os
import socketserver

os.chdir(os.path.dirname(os.path.abspath(__file__)))
PORT = int(os.environ.get("PORT", "5500"))


class NoCacheHandler(http.server.SimpleHTTPRequestHandler):
    def end_headers(self):
        self.send_header("Cache-Control", "no-store, must-revalidate")
        self.send_header("Pragma", "no-cache")
        super().end_headers()


class Server(socketserver.TCPServer):
    allow_reuse_address = True


with Server(("", PORT), NoCacheHandler) as httpd:
    print(f"frontend (no-cache) serving on http://localhost:{PORT}")
    httpd.serve_forever()

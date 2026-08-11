const http = require('http');
const fs = require('fs');
const path = require('path');

const PORT = process.env.PORT || 8000;
const ROOT_DIR = path.resolve(__dirname);
const ADMIN_DIST = path.join(ROOT_DIR, 'admin-portal');

const mimeTypes = {
  '.html': 'text/html',
  '.css': 'text/css',
  '.js': 'application/javascript',
  '.json': 'application/json',
  '.svg': 'image/svg+xml',
  '.png': 'image/png',
  '.jpg': 'image/jpeg',
  '.jpeg': 'image/jpeg',
  '.ico': 'image/x-icon',
  '.woff2': 'font/woff2',
  '.woff': 'font/woff',
  '.ttf': 'font/ttf',
};

function getFilePath(urlPath) {
  if (urlPath === '/admin-portal') {
    return { redirect: '/admin-portal/' };
  }

  if (urlPath.startsWith('/admin-portal/')) {
    const subPath = urlPath.replace('/admin-portal/', '');
    const safePath = subPath ? path.normalize(subPath) : 'index.html';
    const targetPath = path.join(ADMIN_DIST, safePath);
    return { file: targetPath };
  }

  const safePath = urlPath === '/' ? 'index.html' : path.normalize(urlPath.replace(/^\//, ''));
  return { file: path.join(ROOT_DIR, safePath) };
}

function sendFile(res, filePath) {
  const ext = path.extname(filePath).toLowerCase();
  const contentType = mimeTypes[ext] || 'application/octet-stream';

  fs.readFile(filePath, (err, content) => {
    if (err) {
      res.writeHead(404, { 'Content-Type': 'text/plain' });
      res.end('404 Not Found');
      return;
    }

    res.writeHead(200, { 'Content-Type': contentType });
    res.end(content);
  });
}

const server = http.createServer((req, res) => {
  const requestUrl = decodeURIComponent(req.url.split('?')[0]);
  const { redirect, file } = getFilePath(requestUrl);

  if (redirect) {
    res.writeHead(302, { Location: redirect });
    res.end();
    return;
  }

  if (!file || !file.startsWith(ROOT_DIR) && !file.startsWith(ADMIN_DIST)) {
    res.writeHead(400, { 'Content-Type': 'text/plain' });
    res.end('400 Bad Request');
    return;
  }

  const resolvedPath = path.resolve(file);

  fs.stat(resolvedPath, (err, stats) => {
    if (err) {
      res.writeHead(404, { 'Content-Type': 'text/plain' });
      res.end('404 Not Found');
      return;
    }

    if (stats.isDirectory()) {
      sendFile(res, path.join(resolvedPath, 'index.html'));
    } else {
      sendFile(res, resolvedPath);
    }
  });
});

server.listen(PORT, () => {
  console.log(`Static server running at http://localhost:${PORT}`);
  console.log(`Root site: http://localhost:${PORT}/`);
  console.log(`Admin portal: http://localhost:${PORT}/admin-portal/`);
});

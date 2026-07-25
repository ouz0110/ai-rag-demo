import { defineConfig, loadEnv } from 'vite';
import vue from '@vitejs/plugin-vue';
import path from 'path';

// https://vitejs.dev/config/
export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '');
  const targetHost = env.VITE_BACKEND_URL || 'http://localhost:8001';

  return {
    plugins: [vue()],
    resolve: {
      alias: {
        '@': path.resolve(__dirname, './src'),
      },
    },
    server: {
      port: 3000,
      open: true,
      proxy: {
        '/base': {
          target: targetHost,
          changeOrigin: true,
          configure: (proxy) => {
            proxy.on('error', (err, _req, res) => {
              console.error(`[Vite Proxy Error] 无法连接到后端服务 (${targetHost}):`, err.message);
              const httpRes = res as any;
              if (httpRes && !httpRes.headersSent && typeof httpRes.writeHead === 'function') {
                httpRes.writeHead(502, { 'Content-Type': 'application/json; charset=utf-8' });
                httpRes.end(
                  JSON.stringify({
                    code: 502,
                    message: `前端无法连接到后端服务 (${targetHost})，请确保 Go 后端服务已成功启动 (如 go run ./cmd/server)`,
                  })
                );
              }
            });
          },
        },
        '/nocli': {
          target: targetHost,
          changeOrigin: true,
          configure: (proxy) => {
            proxy.on('error', (err, _req, res) => {
              console.error(`[Vite Proxy Error] 无法连接到后端服务 (${targetHost}):`, err.message);
              const httpRes = res as any;
              if (httpRes && !httpRes.headersSent && typeof httpRes.writeHead === 'function') {
                httpRes.writeHead(502, { 'Content-Type': 'application/json; charset=utf-8' });
                httpRes.end(
                  JSON.stringify({
                    code: 502,
                    message: `前端无法连接到后端服务 (${targetHost})，请确保 Go 后端服务已成功启动 (如 go run ./cmd/server)`,
                  })
                );
              }
            });
          },
        },
      },
    },
  };
});

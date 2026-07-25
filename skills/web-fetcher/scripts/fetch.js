#!/usr/bin/env node

/**
 * fetch.js - 网页信息抓取脚本
 * 用法: node fetch.js <URL> [max_bytes]
 */

const http = require('http');
const https = require('https');
const { URL } = require('url');

const targetUrl = process.argv[2];
const maxBytes = parseInt(process.argv[3] || '1048576', 10); // 默认最大 1MB

if (!targetUrl) {
    console.error('错误: 请提供目标网页 URL。用法: node fetch.js <URL>');
    process.exit(1);
}

function fetchUrl(urlStr, redirectCount = 0) {
    if (redirectCount > 5) {
        console.error('错误: 重定向次数过多 (>5)');
        process.exit(1);
    }

    let parsedUrl;
    try {
        parsedUrl = new URL(urlStr);
    } catch (e) {
        console.error(`错误: URL 格式非法: ${urlStr}`);
        process.exit(1);
    }

    const client = parsedUrl.protocol === 'https:' ? https : http;
    const options = {
        headers: {
            'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36',
            'Accept': 'text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8',
            'Accept-Language': 'zh-CN,zh;q=0.9,en;q=0.8'
        }
    };

    const req = client.get(parsedUrl, options, (res) => {
        // 处理重定向 301, 302, 307, 308
        if ([301, 302, 303, 307, 308].includes(res.statusCode) && res.headers.location) {
            const redirectUrl = new URL(res.headers.location, parsedUrl).toString();
            return fetchUrl(redirectUrl, redirectCount + 1);
        }

        if (res.statusCode < 200 || res.statusCode >= 300) {
            console.error(`HTTP 错误: 状态码 ${res.statusCode} (${res.statusMessage})`);
            process.exit(1);
        }

        let body = '';
        let totalBytes = 0;

        res.setEncoding('utf8');
        res.on('data', (chunk) => {
            totalBytes += Buffer.byteLength(chunk, 'utf8');
            if (totalBytes <= maxBytes) {
                body += chunk;
            }
        });

        res.on('end', () => {
            if (totalBytes > maxBytes) {
                body += '\n\n... [输出已超出最大字节限制并截断]';
            }
            console.log(body);
        });
    });

    req.on('error', (err) => {
        console.error(`网络请求失败: ${err.message}`);
        process.exit(1);
    });

    req.setTimeout(15000, () => {
        req.destroy();
        console.error('请求超时 (15 秒)');
        process.exit(1);
    });
}

fetchUrl(targetUrl);

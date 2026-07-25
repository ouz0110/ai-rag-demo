import type { StreamChunk } from '../types/api';

export interface SSEClientOptions {
  url: string;
  body: Record<string, any>;
  onChunk: (chunk: StreamChunk) => void;
  onError?: (err: Error) => void;
  onDone?: () => void;
  signal?: AbortSignal;
}

export async function fetchSSE(options: SSEClientOptions): Promise<void> {
  const token = localStorage.getItem('auth_token');
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    'Accept': 'text/event-stream',
  };

  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }

  try {
    const response = await fetch(options.url, {
      method: 'POST',
      headers,
      body: JSON.stringify(options.body),
      signal: options.signal,
    });

    if (!response.ok) {
      let errorMsg = `HTTP Error ${response.status}: ${response.statusText}`;
      try {
        const errJson = await response.json();
        if (errJson.message) errorMsg = errJson.message;
      } catch (e) {
        // ignore
      }
      throw new Error(errorMsg);
    }

    if (!response.body) {
      throw new Error('Response body is null, SSE stream unavailable');
    }

    const reader = response.body.getReader();
    const decoder = new TextDecoder('utf-8');
    let buffer = '';

    while (true) {
      const { done, value } = await reader.read();
      if (done) break;

      buffer += decoder.decode(value, { stream: true });

      // 按 \n\n 切分完整的 SSE 事件帧
      const parts = buffer.split('\n\n');
      // 保留最后一个可能未接收完整的片段
      buffer = parts.pop() || '';

      for (const part of parts) {
        if (!part.trim()) continue;

        let currentDataStr = '';
        const lines = part.split('\n');

        for (const line of lines) {
          const trimmed = line.trim();
          if (trimmed.startsWith('data:')) {
            currentDataStr = trimmed.substring(5).trim();
          }
        }

        if (currentDataStr) {
          try {
            const chunk: StreamChunk = JSON.parse(currentDataStr);
            options.onChunk(chunk);
          } catch (err) {
            console.error('Failed to parse SSE chunk JSON:', currentDataStr, err);
          }
        }
      }
    }

    // 处理残留在 buffer 中的最后数据
    if (buffer.trim()) {
      const lines = buffer.split('\n');
      let currentDataStr = '';
      for (const line of lines) {
        const trimmed = line.trim();
        if (trimmed.startsWith('data:')) {
          currentDataStr = trimmed.substring(5).trim();
        }
      }
      if (currentDataStr) {
        try {
          const chunk: StreamChunk = JSON.parse(currentDataStr);
          options.onChunk(chunk);
        } catch (e) {
          // ignore
        }
      }
    }

    if (options.onDone) {
      options.onDone();
    }
  } catch (err: any) {
    if (err.name === 'AbortError') {
      console.log('SSE Stream request aborted by user');
      if (options.onDone) options.onDone();
      return;
    }
    if (options.onError) {
      options.onError(err);
    } else {
      console.error('SSE connection error:', err);
    }
  }
}

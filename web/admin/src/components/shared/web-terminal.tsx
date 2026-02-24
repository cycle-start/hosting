import { useEffect, useRef } from 'react'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import { WebLinksAddon } from '@xterm/addon-web-links'
import '@xterm/xterm/css/xterm.css'
import { api } from '@/lib/api'

interface WebTerminalProps {
  tenantId: string
  onClose?: () => void
}

export function WebTerminal({ tenantId, onClose }: WebTerminalProps) {
  const containerRef = useRef<HTMLDivElement>(null)
  const termRef = useRef<Terminal | null>(null)
  const wsRef = useRef<WebSocket | null>(null)

  useEffect(() => {
    if (!containerRef.current) return

    const term = new Terminal({
      cursorBlink: true,
      fontSize: 14,
      fontFamily: 'Menlo, Monaco, "Courier New", monospace',
      theme: {
        background: '#1a1b26',
        foreground: '#a9b1d6',
        cursor: '#c0caf5',
      },
    })
    termRef.current = term

    const fitAddon = new FitAddon()
    term.loadAddon(fitAddon)
    term.loadAddon(new WebLinksAddon())

    term.open(containerRef.current)
    fitAddon.fit()

    let cancelled = false
    let ws: WebSocket | null = null
    let resizeObserver: ResizeObserver | null = null

    const token = api.getApiKey()
    const terminalPath = `/api/v1/tenants/${tenantId}/terminal`
    const tokenParam = `token=${encodeURIComponent(token || '')}`

    // Preflight check — hit the endpoint via HTTP first so we can read error responses.
    // The server will return 426 Upgrade Required for a valid request (meaning WebSocket is expected).
    fetch(`${terminalPath}?${tokenParam}`)
      .then(async (res) => {
        if (cancelled) return
        // 426 = server wants a WebSocket upgrade, which means the endpoint is reachable and auth passed.
        if (res.status === 426) return connectWebSocket()
        const body = await res.json().catch(() => ({ error: res.statusText }))
        term.write(`\x1b[31mTerminal error: ${body.error || res.statusText} (${res.status})\x1b[0m\r\n`)
      })
      .catch((err) => {
        if (!cancelled) term.write(`\x1b[31mTerminal error: ${err.message}\x1b[0m\r\n`)
      })

    function connectWebSocket() {
      const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
      const wsUrl = `${protocol}//${window.location.host}${terminalPath}?${tokenParam}`

      ws = new WebSocket(wsUrl)
      ws.binaryType = 'arraybuffer'
      wsRef.current = ws

      ws.onopen = () => {
        ws!.send(JSON.stringify({ type: 'resize', cols: term.cols, rows: term.rows }))
      }

      ws.onmessage = (event) => {
        if (event.data instanceof ArrayBuffer) {
          term.write(new Uint8Array(event.data))
        } else {
          term.write(event.data)
        }
      }

      ws.onclose = (event) => {
        if (event.reason) {
          term.write(`\r\n\x1b[31m${event.reason}\x1b[0m\r\n`)
        } else {
          term.write('\r\n\x1b[33mConnection closed.\x1b[0m\r\n')
        }
      }

      ws.onerror = () => {
        term.write('\r\n\x1b[31mConnection error.\x1b[0m\r\n')
      }

      // Terminal input -> WebSocket (binary).
      term.onData((data) => {
        if (ws?.readyState === WebSocket.OPEN) {
          ws.send(new TextEncoder().encode(data))
        }
      })

      // Handle resize.
      resizeObserver = new ResizeObserver(() => {
        fitAddon.fit()
      })
      resizeObserver.observe(containerRef.current!)

      term.onResize(({ cols, rows }) => {
        if (ws?.readyState === WebSocket.OPEN) {
          ws.send(JSON.stringify({ type: 'resize', cols, rows }))
        }
      })
    }

    return () => {
      cancelled = true
      resizeObserver?.disconnect()
      ws?.close()
      term.dispose()
      termRef.current = null
      wsRef.current = null
    }
  }, [tenantId])

  return (
    <div
      ref={containerRef}
      className="w-full h-[700px]"
      style={{ backgroundColor: '#1a1b26' }}
    />
  )
}

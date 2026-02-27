import { useEffect, useRef } from "react";
import { Terminal } from "@xterm/xterm";
import { FitAddon } from "@xterm/addon-fit";
import { WebLinksAddon } from "@xterm/addon-web-links";
import { useAuth } from "@/auth/context";
import "@xterm/xterm/css/xterm.css";

export function WebTerminal({ webrootId }: { webrootId: string }) {
  const containerRef = useRef<HTMLDivElement>(null);
  const { token } = useAuth();

  useEffect(() => {
    const el = containerRef.current;
    if (!el || !token) return;

    const term = new Terminal({
      cursorBlink: true,
      fontFamily: "Menlo, Monaco, 'Courier New', monospace",
      fontSize: 14,
      theme: {
        background: "#1a1b26",
        foreground: "#a9b1d6",
        cursor: "#c0caf5",
        selectionBackground: "#33467c",
        black: "#15161e",
        red: "#f7768e",
        green: "#9ece6a",
        yellow: "#e0af68",
        blue: "#7aa2f7",
        magenta: "#bb9af7",
        cyan: "#7dcfff",
        white: "#a9b1d6",
        brightBlack: "#414868",
        brightRed: "#f7768e",
        brightGreen: "#9ece6a",
        brightYellow: "#e0af68",
        brightBlue: "#7aa2f7",
        brightMagenta: "#bb9af7",
        brightCyan: "#7dcfff",
        brightWhite: "#c0caf5",
      },
    });

    const fitAddon = new FitAddon();
    term.loadAddon(fitAddon);
    term.loadAddon(new WebLinksAddon());
    term.open(el);
    fitAddon.fit();

    const proto = window.location.protocol === "https:" ? "wss:" : "ws:";
    const host = import.meta.env.VITE_API_URL
      ? new URL(import.meta.env.VITE_API_URL).host
      : window.location.host;
    const wsUrl = `${proto}//${host}/api/v1/webroots/${webrootId}/terminal?token=${encodeURIComponent(token)}`;

    const ws = new WebSocket(wsUrl);
    ws.binaryType = "arraybuffer";

    ws.addEventListener("open", () => {
      // Send initial resize
      const dims = { cols: term.cols, rows: term.rows };
      ws.send(JSON.stringify({ type: "resize", ...dims }));
    });

    ws.addEventListener("message", (ev) => {
      if (ev.data instanceof ArrayBuffer) {
        term.write(new Uint8Array(ev.data));
      } else {
        term.write(ev.data);
      }
    });

    ws.addEventListener("close", () => {
      term.write("\r\n\x1b[90m[Connection closed]\x1b[0m\r\n");
    });

    // Terminal input → WS as binary
    const onData = term.onData((data) => {
      if (ws.readyState === WebSocket.OPEN) {
        ws.send(new TextEncoder().encode(data));
      }
    });

    // Resize handling
    const onResize = term.onResize(({ cols, rows }) => {
      if (ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({ type: "resize", cols, rows }));
      }
    });

    const observer = new ResizeObserver(() => {
      fitAddon.fit();
    });
    observer.observe(el);

    return () => {
      observer.disconnect();
      onData.dispose();
      onResize.dispose();
      ws.close();
      term.dispose();
    };
  }, [webrootId, token]);

  return (
    <div
      ref={containerRef}
      className="h-[500px] w-full overflow-hidden rounded-xl ring-1 ring-gray-200"
    />
  );
}

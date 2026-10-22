import { useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import { toast } from "sonner";
import { ArrowDownToLine, Download, Eraser, Pause, Play, Search } from "@/components/ui/lucide";
import { api } from "@/lib/api";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { OptionSelect } from "@/components/ui/option-select";

interface Line {
  n: number;
  t: string;
  s: "out" | "err";
  l: string;
}

const MAX = 5000;
const tails = [
  { id: "100", label: "100" },
  { id: "500", label: "500" },
  { id: "2000", label: "2,000" },
  { id: "5000", label: "5,000" },
] as const;
type Tail = (typeof tails)[number]["id"];

/**
 * A container's output, streamed live over Server-Sent Events. Keeps the newest 5,000
 * lines; follows the end unless scrolled up; pausing closes the stream.
 */
export function LogViewer({ id, name }: { id: string; name: string }) {
  const [tail, setTail] = useState<Tail>("500");
  const [live, setLive] = useState(true);
  const [lines, setLines] = useState<Line[]>([]);
  const [status, setStatus] = useState<"connecting" | "open" | "ended" | "error">("connecting");
  const [q, setQ] = useState("");
  const [onlyErr, setOnlyErr] = useState(false);
  const [times, setTimes] = useState(false);
  const [wrap, setWrap] = useState(true);
  const [stick, setStick] = useState(true);
  const box = useRef<HTMLDivElement>(null);
  const counter = useRef(0);

  useEffect(() => {
    if (!live) return;
    setLines([]);
    setStatus("connecting");
    const es = new EventSource(api.logsURL(id, Number(tail), true));
    // Lines arrive in bursts (the tail at once); batch them per frame.
    let pending: Line[] = [];
    let raf = 0;
    const flush = () => {
      raf = 0;
      const add = pending;
      pending = [];
      setLines((prev) => {
        const next = prev.concat(add);
        return next.length > MAX ? next.slice(next.length - MAX) : next;
      });
    };
    // EventSource reconnects by itself after a drop and the tail is sent again, so start over.
    es.onopen = () => {
      pending = [];
      setLines([]);
      setStatus("open");
    };
    es.onmessage = (e) => {
      const m = JSON.parse(e.data) as Omit<Line, "n">;
      pending.push({ ...m, n: ++counter.current });
      if (!raf) raf = requestAnimationFrame(flush);
    };
    es.addEventListener("end", () => {
      setStatus("ended");
      es.close();
    });
    es.onerror = () => {
      if (es.readyState === EventSource.CLOSED) setStatus("error");
    };
    return () => {
      es.close();
      if (raf) cancelAnimationFrame(raf);
    };
  }, [id, tail, live]);

  const shown = useMemo(() => {
    const needle = q.trim().toLowerCase();
    return lines.filter((l) => (!onlyErr || l.s === "err") && (!needle || l.l.toLowerCase().includes(needle)));
  }, [lines, q, onlyErr]);

  useLayoutEffect(() => {
    if (stick && box.current) box.current.scrollTop = box.current.scrollHeight;
  }, [shown, stick]);

  function download() {
    const text = lines.map((l) => `${l.t} ${l.s === "err" ? "E" : " "} ${l.l}`).join("\n");
    const url = URL.createObjectURL(new Blob([text], { type: "text/plain" }));
    const a = document.createElement("a");
    a.href = url;
    a.download = `${name}-${new Date().toISOString().replace(/[:.]/g, "-")}.log`;
    a.click();
    URL.revokeObjectURL(url);
    toast.success(`Saved ${lines.length.toLocaleString()} lines`, { description: a.download });
  }

  const toggle = "h-7 rounded-md px-2.5 text-[0.8125rem] transition-colors aria-pressed:bg-sidebar-active aria-pressed:font-medium aria-pressed:text-foreground text-muted-foreground hover:text-foreground";
  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-wrap items-center gap-2">
        <div className="relative min-w-48 flex-1">
          <Search className="pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-muted-foreground" aria-hidden />
          <input
            value={q}
            onChange={(e) => setQ(e.target.value)}
            placeholder="Filter lines"
            aria-label="Filter log lines"
            className="h-8 w-full rounded-md border border-input bg-card pr-3 pl-8 text-sm shadow-xs outline-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50"
          />
        </div>
        <OptionSelect label="Lines to load" value={tail} onChange={setTail} options={[...tails]} />
        <div className="flex items-center gap-0.5 rounded-lg bg-muted p-0.5 dark:bg-muted/60">
          <button type="button" className={toggle} aria-pressed={onlyErr} onClick={() => setOnlyErr((v) => !v)}>
            stderr only
          </button>
          <button type="button" className={toggle} aria-pressed={times} onClick={() => setTimes((v) => !v)}>
            Times
          </button>
          <button type="button" className={toggle} aria-pressed={wrap} onClick={() => setWrap((v) => !v)}>
            Wrap
          </button>
        </div>
        <Button
          size="sm"
          variant="outline"
          icon={live ? Pause : Play}
          onClick={() => {
            setLive((v) => !v);
            toast(live ? "Log stream paused" : "Log stream resumed");
          }}
        >
          {live ? "Pause" : "Resume"}
        </Button>
        <Button
          size="icon-sm"
          variant="ghost"
          onClick={() => {
            setLines([]);
            toast("Cleared the view", { description: "New lines keep arriving." });
          }}
          aria-label="Clear"
          title="Clear"
        >
          <Eraser aria-hidden />
        </Button>
        <Button size="icon-sm" variant="ghost" onClick={download} aria-label="Download" title="Download what's loaded">
          <Download aria-hidden />
        </Button>
      </div>
      <div className="relative">
        <div
          ref={box}
          onScroll={(e) => {
            const el = e.currentTarget;
            setStick(el.scrollHeight - el.scrollTop - el.clientHeight < 24);
          }}
          className="h-[min(65vh,640px)] overflow-auto rounded-md border bg-[#141412] p-3 font-mono text-[12px] leading-[1.55] text-[#e8e6df]"
          role="log"
          aria-live="off"
          aria-label={`${name} logs`}
        >
          {shown.length === 0 ? (
            <p className="text-[#a6a49c]">{status === "connecting" ? "Connecting…" : lines.length ? "No line matches." : "No output yet."}</p>
          ) : (
            shown.map((l) => (
              <div key={l.n} className={cn("flex gap-3", wrap ? "whitespace-pre-wrap break-all" : "whitespace-pre", l.s === "err" && "text-[#ff9c8a]")}>
                {times && <span className="shrink-0 text-[#7d7b74] select-none">{l.t.slice(0, 19).replace("T", " ")}</span>}
                <span className="min-w-0">{l.l || " "}</span>
              </div>
            ))
          )}
        </div>
        {!stick && (
          <Button size="sm" className="absolute right-4 bottom-4 shadow-md" icon={ArrowDownToLine} onClick={() => setStick(true)}>
            Jump to end
          </Button>
        )}
      </div>
      <p className="text-xs text-muted-foreground">
        {status === "open" && live ? "Streaming live. " : status === "ended" ? "The container stopped; the stream ended. " : status === "error" ? "The stream dropped. Resume to reconnect. " : !live ? "Paused. " : ""}
        {lines.length.toLocaleString()} lines loaded{q || onlyErr ? `, ${shown.length.toLocaleString()} shown` : ""}.
      </p>
    </div>
  );
}

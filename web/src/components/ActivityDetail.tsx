import type { ReactNode } from "react";
import { useQuery } from "@tanstack/react-query";
import { toast } from "sonner";
import { Copy, MapPin } from "@/components/ui/lucide";
import { api, type Activity, type ActivityDetail as Detail, type Located } from "@/lib/api";
import { formatFullDateTime, formatTime } from "@/lib/format";
import { SimpleTable, type SimpleColumn } from "@/components/ui/simple-table";
import { Skeleton } from "@/components/ui/skeleton";

/** The panel under an opened Activity row: every address behind it, with where it is. */
export function ActivityDetail({ entry }: { entry: Activity }) {
  const q = useQuery({ queryKey: ["activity", "detail", entry.id], queryFn: () => api.activityDetail(entry.id), staleTime: 60_000 });
  if (q.error) return <p className="pt-2 text-sm text-critical">{q.error.message}</p>;
  if (!q.data) return <Skeleton className="mt-2 h-24 w-full" />;
  const d = q.data;
  const summed = d.kind === "host" && d.type === "ssh" && d.action === "failed logins";
  return (
    <div className="flex flex-col gap-3 pt-2">
      {summed ? <FailedLogins d={d} /> : <Single d={d} />}
      <p className="text-xs text-muted-foreground">
        {d.sources.length === 0 ? null : d.geo ? (
          <>
            IP location by{" "}
            <a href="https://db-ip.com" target="_blank" rel="noreferrer" className="underline underline-offset-2 hover:text-foreground">
              DB-IP
            </a>
            , city-level and approximate.
          </>
        ) : (
          "Locations show once the location database finishes downloading, a few minutes after StunSails starts."
        )}
      </p>
    </div>
  );
}

function FailedLogins({ d }: { d: Detail }) {
  const attempts = d.sources.reduce((n, s) => n + (s.attempts ?? 0), 0);
  // Entries written before StunSails kept every address only name the top three.
  const partial = /\+\d+ more/.test(d.detail ?? "") && d.sources.length <= 3;
  // A summary covers the ten minutes before it was written.
  const from = (d.at - 600) * 1000;
  const columns: SimpleColumn<Located>[] = [
    { header: "Address", cell: (s) => <Address ip={s.ip} /> },
    { header: "Attempts", align: "right", className: "num", cell: (s) => s.attempts?.toLocaleString() ?? "—" },
    { header: "Location", cell: (s) => <Place s={s} /> },
    { header: "Network", className: "text-muted-foreground", cell: (s) => network(s) },
    {
      header: "Users tried",
      className: "max-w-[16rem] truncate text-muted-foreground",
      cell: (s) => <span title={s.users?.join(", ")}>{s.users?.length ? s.users.join(", ") : "—"}</span>,
    },
  ];
  return (
    <>
      <p className="text-sm">
        {partial ? (
          <>
            {d.name} between <span className="num">{formatTime(from)}</span> and <span className="num">{formatTime(d.at * 1000)}</span>. This entry is from before StunSails kept every address, so
            only the busiest three are listed.
          </>
        ) : (
          <>
            <span className="num">{attempts.toLocaleString()}</span> wrong {attempts === 1 ? "try" : "tries"} from <span className="num">{d.sources.length}</span>{" "}
            {d.sources.length === 1 ? "address" : "addresses"} between <span className="num">{formatTime(from)}</span> and <span className="num">{formatTime(d.at * 1000)}</span>.
          </>
        )}
      </p>
      <SimpleTable label="Addresses that failed to sign in over SSH" columns={columns} rows={d.sources} rowKey={(s) => s.ip} empty="No addresses were recorded for this entry." />
    </>
  );
}

function Single({ d }: { d: Detail }) {
  const s = d.sources[0];
  const rows: [string, ReactNode][] = [["When", <span className="num">{formatFullDateTime(d.at * 1000)}</span>]];
  if (s) {
    rows.push(["Address", <Address ip={s.ip} />], ["Location", <Place s={s} />], ["Network", network(s)]);
  }
  if (d.target && d.target !== d.name) rows.push(["Target", <span className="num break-all">{d.target}</span>]);
  if (d.type === "sudo") {
    rows.push(["Command", <code className="block rounded-md border bg-card px-2 py-1.5 font-mono text-xs break-all whitespace-pre-wrap">{d.detail}</code>]);
  } else if (d.detail) {
    rows.push(["Detail", <span className="break-words">{d.detail}</span>]);
  }
  return (
    <dl className="grid grid-cols-[minmax(6rem,auto)_1fr] gap-x-6 gap-y-2 text-sm">
      {rows.map(([k, v]) => (
        <div key={k} className="contents">
          <dt className="text-muted-foreground">{k}</dt>
          <dd className="min-w-0">{v}</dd>
        </div>
      ))}
    </dl>
  );
}

function Address({ ip }: { ip: string }) {
  return (
    <span className="inline-flex items-center gap-1.5">
      <span className="num">{ip}</span>
      <button
        type="button"
        aria-label={`Copy ${ip}`}
        onClick={() =>
          navigator.clipboard.writeText(ip).then(
            () => toast.success(`Copied ${ip}`),
            () => toast.error("Couldn't copy: the browser blocked the clipboard"),
          )
        }
        className="grid size-6 place-items-center rounded text-muted-foreground hover:text-foreground focus-visible:ring-[3px] focus-visible:ring-ring/50 focus-visible:outline-none"
      >
        <Copy className="size-3.5" />
      </button>
    </span>
  );
}

function Place({ s }: { s: Located }) {
  if (s.private) return <span className="text-muted-foreground">Private network</span>;
  if (!s.country) return <span className="text-muted-foreground">Unknown</span>;
  const place = [s.city, s.region !== s.city ? s.region : undefined, s.country].filter(Boolean).join(", ");
  return (
    <span className="inline-flex items-center gap-1.5 whitespace-nowrap">
      {s.country_code ? (
        <span aria-hidden className="text-base leading-none">
          {flag(s.country_code)}
        </span>
      ) : (
        <MapPin className="size-3.5 text-muted-foreground" aria-hidden />
      )}
      {place}
    </span>
  );
}

function network(s: Located) {
  if (s.private) return "—";
  if (!s.asn) return s.org || "—";
  return `AS${s.asn}${s.org ? ` ${s.org}` : ""}`;
}

/** "ID" as 🇮🇩: two regional-indicator letters. */
function flag(code: string) {
  if (!/^[A-Za-z]{2}$/.test(code)) return "";
  return String.fromCodePoint(...[...code.toUpperCase()].map((c) => 0x1f1a5 + c.charCodeAt(0)));
}

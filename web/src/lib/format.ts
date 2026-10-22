// Number, money, date and duration formats used in more than one place live here;
// a format only one view needs (a page heading, say) stays in that view.

const usd = new Intl.NumberFormat("en-US", { style: "currency", currency: "USD", minimumFractionDigits: 2, maximumFractionDigits: 4 });

/** AI spend: cents matter, fractions of a cent read as "<$0.01". */
export function formatUSD(value: number) {
  if (value === 0) return "$0.00";
  if (value < 0.01) return "<$0.01";
  return usd.format(value);
}

const moneyFormats = new Map<string, Intl.NumberFormat>();

/** Prices and budgets: whole units unless decimals is given. An unknown currency code is shown as-is. */
export function formatMoney(value: number, currency = "USD", decimals = 0) {
  const code = currency || "USD";
  const key = `${code}:${decimals}`;
  let f = moneyFormats.get(key);
  if (!f) {
    try {
      f = new Intl.NumberFormat("en-US", { style: "currency", currency: code, minimumFractionDigits: decimals, maximumFractionDigits: decimals });
    } catch {
      return `${code} ${value.toLocaleString("en-US", { maximumFractionDigits: decimals })}`;
    }
    moneyFormats.set(key, f);
  }
  return f.format(value);
}

export function formatDate(iso: string) {
  return new Date(iso).toLocaleDateString(undefined, { day: "numeric", month: "short", year: "numeric" });
}

export function formatDateTime(iso: string | number) {
  return new Date(iso).toLocaleString(undefined, { day: "numeric", month: "short", hour: "2-digit", minute: "2-digit" });
}

/** The exact moment, for tooltips and labels: "8 Oct 2026, 14:05". */
export function formatFullDateTime(iso: string | number) {
  return new Date(iso).toLocaleString(undefined, { dateStyle: "medium", timeStyle: "short" });
}

/** A calendar day ("2026-10-08") as "8 Oct", read in local time. */
export function formatDay(day: string) {
  return new Date(day + "T00:00:00").toLocaleDateString(undefined, { day: "numeric", month: "short" });
}

export function formatTime(iso: string | number) {
  return new Date(iso).toLocaleTimeString(undefined, { hour: "2-digit", minute: "2-digit" });
}

/** "just now", "5 min ago", … relative to now (pass useNow() in components). */
export function formatRelative(iso: string, now = Date.now()) {
  const seconds = (now - new Date(iso).getTime()) / 1000;
  if (seconds < 60) return "just now";
  if (seconds < 3600) return `${Math.floor(seconds / 60)} min ago`;
  if (seconds < 86400) return `${Math.floor(seconds / 3600)} h ago`;
  if (seconds < 7 * 86400) return `${Math.floor(seconds / 86400)} d ago`;
  return formatDate(iso);
}

/** A running time as "42s", "3m 05s" or "1h 12m". */
export function formatElapsed(ms: number) {
  const s = Math.max(0, Math.floor(ms / 1000));
  if (s < 60) return `${s}s`;
  if (s < 3600) return `${Math.floor(s / 60)}m ${String(s % 60).padStart(2, "0")}s`;
  return `${Math.floor(s / 3600)}h ${Math.floor((s % 3600) / 60)}m`;
}

const counts = new Intl.NumberFormat("en-US");
const compactCounts = new Intl.NumberFormat("en-US", { notation: "compact", maximumFractionDigits: 1 });

/** A count with thousands separators: "12,345". */
export function formatCount(n: number) {
  return counts.format(n);
}

/** A large count, shortened: "12.3K", "4.5M". */
export function formatCompact(n: number) {
  return compactCounts.format(n);
}

export function formatBytes(bytes: number) {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(0)} KB`;
  if (bytes < 1024 ** 3) return `${(bytes / 1024 / 1024).toFixed(1)} MB`;
  return `${(bytes / 1024 ** 3).toFixed(1)} GB`;
}

/** "1 file", "3 files": a count with its noun. */
export function plural(n: number, one: string, many = one + "s") {
  return `${n} ${n === 1 ? one : many}`;
}

/** Unix seconds as "5 min ago", for API timestamps. */
export function ago(unix: number, now = Date.now()) {
  return formatRelative(new Date(unix * 1000).toISOString(), now);
}

/** Throughput: "0 B/s", "12 KB/s", "3.4 MB/s". */
export function formatRate(bytesPerSec: number) {
  return `${formatBytes(Math.round(bytesPerSec))}/s`;
}

/** A length of time in seconds as "3 d 4 h", "5 h 12 min" or "42 s". */
export function formatDuration(sec: number) {
  const s = Math.max(0, Math.floor(sec));
  const d = Math.floor(s / 86400);
  const h = Math.floor((s % 86400) / 3600);
  const m = Math.floor((s % 3600) / 60);
  if (d > 0) return `${d} d ${h} h`;
  if (h > 0) return `${h} h ${m} min`;
  if (m > 0) return `${m} min`;
  return `${s} s`;
}

/** Percent of total, rounded; 0 when total is unknown. */
export const pct = (used: number, total: number) => (total > 0 ? Math.round((used / total) * 100) : 0);

/** The first 12 characters of a Docker id, as the CLI shows it. */
export const shortId = (id: string) => id.replace(/^sha256:/, "").slice(0, 12);

/** An image reference without its digest, and long hash tags cut to 12: what fits a row. */
export function shortImage(ref: string) {
  if (ref.startsWith("sha256:")) return shortId(ref);
  const [name = ref] = ref.split("@");
  const i = name.lastIndexOf(":");
  if (i > name.lastIndexOf("/") && name.length - i > 20) return `${name.slice(0, i)}:${name.slice(i + 1, i + 13)}`;
  return name;
}

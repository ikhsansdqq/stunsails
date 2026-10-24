// StunSails' API: JSON over same-origin fetch with the session cookie. Every change
// carries X-StunSails (the server refuses changes without it) and may answer
// "stepup_required", which the StepUp provider turns into a code prompt.

export class TimeoutError extends Error {
  constructor() {
    super("The server took too long to answer.");
  }
}

export class ApiError extends Error {
  constructor(
    message: string,
    public status: number,
    public code?: string,
  ) {
    super(message);
  }
}

async function request<T>(method: string, path: string, body?: unknown, timeoutMs = 30_000): Promise<T> {
  const ctl = new AbortController();
  const timer = setTimeout(() => ctl.abort(), timeoutMs);
  let res: Response;
  try {
    res = await fetch(path, {
      method,
      credentials: "same-origin",
      headers: { ...(body !== undefined ? { "Content-Type": "application/json" } : {}), ...(method !== "GET" ? { "X-StunSails": "1" } : {}) },
      body: body !== undefined ? JSON.stringify(body) : undefined,
      signal: ctl.signal,
    });
  } catch (e) {
    if (ctl.signal.aborted) throw new TimeoutError();
    throw e;
  } finally {
    clearTimeout(timer);
  }
  const data = await res.json().catch(() => ({}));
  if (!res.ok) {
    if (res.status === 401 && path !== "/api/login" && path !== "/api/login/totp" && path !== "/api/stepup") {
      dispatchEvent(new Event("stunsails:signed-out"));
    }
    throw new ApiError(data.error || res.statusText, res.status, data.code);
  }
  return data as T;
}

const get = <T>(path: string) => request<T>("GET", path);
const post = <T>(path: string, body?: unknown, timeoutMs?: number) => request<T>("POST", path, body, timeoutMs);
const del = <T>(path: string) => request<T>("DELETE", path);
const put = <T>(path: string, body: unknown) => request<T>("PUT", path, body);
const enc = encodeURIComponent;

export interface HostSample {
  at: number;
  cpu: number;
  load1: number;
  cpus: number;
  mem_used: number;
  mem_total: number;
  swap_used: number;
  swap_total: number;
  disk_used: number;
  disk_total: number;
  net_rx: number;
  net_tx: number;
  disk_read: number;
  disk_write: number;
}

export interface HostInfo {
  hostname: string;
  kernel: string;
  os: string;
  cpus: number;
  cpu_model: string;
  uptime_sec: number;
  load: [number, number, number];
  disks: { mount: string; total: number; used: number; free: number }[];
}

export interface Port {
  ip?: string;
  private: number;
  public?: number;
  type: string;
}

export interface Live {
  cpu: number;
  mem: number;
  mem_limit: number;
  io_read: number;
  io_write: number;
  pids: number;
}

export type ContainerState = "running" | "exited" | "restarting" | "paused" | "created" | "dead" | "removing";

export interface Container {
  id: string;
  name: string;
  image: string;
  image_id: string;
  command: string;
  created: number;
  state: ContainerState;
  status: string;
  health: "" | "healthy" | "unhealthy" | "starting";
  ports: Port[];
  project: string;
  service: string;
  networks: string[];
  live?: Live | null;
  looping?: boolean;
}

export interface Detail {
  id: string;
  name: string;
  image: string;
  image_id: string;
  created: string;
  command: string[];
  state: {
    status: ContainerState;
    running: boolean;
    paused: boolean;
    restarting: boolean;
    oom_killed: boolean;
    pid: number;
    exit_code: number;
    error: string;
    started_at: string;
    finished_at: string;
    health: { status: string; failing_streak: number; log: { start: string; end: string; exit_code: number; output: string }[] } | null;
  };
  restart_count: number;
  restart_policy: string;
  memory_limit: number;
  cpu_limit: number;
  privileged: boolean;
  network_mode: string;
  user: string;
  working_dir: string;
  tty: boolean;
  env_keys: string[];
  labels: Record<string, string>;
  mounts: { type: string; name?: string; source: string; destination: string; rw: boolean }[];
  ports: { container: string; host_ip: string; host_port: string }[];
  networks: { name: string; ip: string; gateway: string; mac: string; aliases: string[] }[];
  project: string;
  service: string;
  healthcheck: string[];
}

export interface CtrPoint {
  at: number;
  cpu: number;
  mem: number;
}

export type StackState = "running" | "partial" | "stopped" | "down";

export interface Stack {
  project: string;
  working_dir: string;
  config_files: string[];
  env_files: string[];
  services: { name: string; containers: Container[] }[];
  running: number;
  total: number;
  unhealthy: number;
  state: StackState;
  last_seen: number;
  manageable: boolean;
  live: Record<string, Live>;
}

export interface Activity {
  id: number;
  at: number;
  kind: "docker" | "audit" | "host" | "alert";
  type: string;
  action: string;
  target: string;
  name: string;
  ip?: string;
  ok: boolean;
  detail?: string;
}

export interface Overview {
  host?: HostSample;
  info: HostInfo;
  docker: { version: string; api_version: string; os: string; arch: string };
  docker_error: string;
  containers: Container[];
  counts: Record<string, number>;
  stacks: { project: string; state: StackState; running: number; total: number; unhealthy: number }[];
  activity: Activity[];
  looping: Container[];
  disk_forecast_days: number;
  failed_units: { unit: string; description: string }[];
  alerts?: ActiveAlert[];
  sites?: { up: number; down: string[] };
}

export interface ActiveAlert {
  rule: string;
  target: string;
  title: string;
  since: number;
}

export interface SmtpConfig {
  enabled: boolean;
  host: string;
  port: number;
  user: string;
  password?: string;
  from: string;
  to: string[];
}

export interface AlertsConfig {
  smtp: SmtpConfig & { password_set: boolean };
  webhook: { enabled: boolean; url_set: boolean; host: string };
  rules: Record<string, boolean>;
  rule_list: { ID: string; Label: string }[];
  active: ActiveAlert[];
}

export interface AlertsUpdate {
  smtp: SmtpConfig;
  webhook: { enabled: boolean; url?: string };
  rules: Record<string, boolean>;
  clear_password?: boolean;
  clear_webhook?: boolean;
}

export type CleanupKind = "build-cache" | "dangling" | "containers" | "networks";

export interface CleanupConfig {
  enabled: boolean;
  weekday: number;
  hour: number;
  kinds: CleanupKind[];
  last_run: number;
  last_result: string;
}

export interface SettingsData {
  alerts: AlertsConfig;
  cleanup: CleanupConfig;
  auth_log_error: string;
}

export interface SiteCheck {
  at: number;
  up: boolean;
  status: number;
  ms: number;
  cert_expiry: number;
  error?: string;
}

export interface Site {
  host: string;
  source: "caddy" | "manual";
  last: SiteCheck | null;
  uptime_24h: number;
}

export interface SelfStats {
  rss: number;
  rss_peak: number;
  threads: number;
  goroutines: number;
  uptime_sec: number;
  go_heap: number;
  go_sys: number;
  mem_current?: number;
  mem_peak?: number;
  mem_max?: number;
  cpu_sec?: number;
  cpu_pct?: number;
  db_bytes?: number;
  binary_bytes?: number;
  helper_image?: string;
  helper_image_bytes: number;
}

export interface Bucket {
  count: number;
  active: number;
  bytes: number;
  reclaimable_bytes: number;
}

export interface DF {
  images: Bucket;
  containers: Bucket;
  volumes: Bucket;
  build_cache: Bucket;
  total_bytes: number;
  reclaimable_bytes: number;
}

export interface Image {
  id: string;
  tags: string[];
  created: number;
  size: number;
  containers: string[];
  dangling: boolean;
}

export interface Volume {
  name: string;
  driver: string;
  mountpoint: string;
  created: string;
  labels: Record<string, string>;
  size: number;
  containers: string[];
  project: string;
}

export interface Network {
  id: string;
  name: string;
  driver: string;
  scope: string;
  internal: boolean;
  subnets: string[];
  created: string;
  containers: string[];
  project: string;
  builtin: boolean;
}

export interface Storage {
  df: DF | null;
  df_error: string;
  measured_at: number;
  images: Image[];
  volumes: Volume[];
  networks: Network[];
  images_error?: string;
  volumes_error?: string;
  networks_error?: string;
}

export interface Session {
  authenticated: boolean;
  configured: boolean;
  stepup_until?: number;
  now?: number;
}

export interface SessionInfo {
  id: string;
  created: number;
  seen: number;
  ip: string;
  ua: string;
  current: boolean;
}

export type Range = "1h" | "24h" | "7d";
export type ContainerAction = "start" | "stop" | "restart" | "kill" | "pause" | "unpause" | "remove";
export type StackVerb = "up" | "down" | "pull" | "start" | "stop" | "restart";
export type PruneKind = "containers" | "images" | "dangling" | "build-cache" | "networks";

/** One address behind a feed line, placed on the map (DB-IP Lite). */
export interface Located {
  ip: string;
  attempts?: number;
  users?: string[];
  country?: string;
  country_code?: string;
  region?: string;
  city?: string;
  asn?: number;
  org?: string;
  private?: boolean;
}

export interface ActivityDetail extends Activity {
  sources: Located[];
  /** Whether the location database is loaded yet. */
  geo: boolean;
}

export interface ActivityPage {
  items: Activity[];
  total: number;
}

export interface ActivityPageQuery extends ActivityQuery {
  offset: number;
  order: "asc" | "desc";
}

export interface ActivityQuery {
  kind?: "" | "docker" | "audit" | "host" | "alert";
  type?: string;
  target?: string;
  q?: string;
  before?: number;
  limit?: number;
}

/** A compose run's output, line by line; resolves when it ends. */
export async function streamStack(project: string, verb: StackVerb, onLine: (line: string) => void): Promise<void> {
  const res = await fetch(`/api/stacks/${enc(project)}/${verb}`, { method: "POST", credentials: "same-origin", headers: { "X-StunSails": "1" } });
  if (!res.ok || !res.body) {
    const data = await res.json().catch(() => ({}));
    throw new ApiError(data.error || res.statusText, res.status, data.code);
  }
  const reader = res.body.pipeThrough(new TextDecoderStream()).getReader();
  let buf = "";
  let result: { ok: boolean; error?: string } | null = null;
  for (;;) {
    const { value, done } = await reader.read();
    if (done) break;
    buf += value;
    let i: number;
    while ((i = buf.indexOf("\n")) >= 0) {
      const raw = buf.slice(0, i);
      buf = buf.slice(i + 1);
      if (!raw) continue;
      const m = JSON.parse(raw) as { t: "out" | "done"; l?: string; ok?: boolean; error?: string };
      if (m.t === "out") onLine(m.l ?? "");
      else result = { ok: !!m.ok, error: m.error };
    }
  }
  if (!result) throw new Error("The connection closed before the action finished. Check the stack's state.");
  if (!result.ok) throw new Error(result.error || "The action failed.");
}

export const api = {
  session: () => get<Session>("/api/session"),
  login: (password: string) => post<{ next: string }>("/api/login", { password }),
  loginCode: (code: string) => post<{ ok: true }>("/api/login/totp", { code }),
  logout: () => post<{ ok: true }>("/api/logout"),
  stepUp: (code: string) => post<{ stepup_until: number }>("/api/stepup", { code }),

  overview: () => get<Overview>("/api/overview"),
  hostHistory: (range: Range) => get<HostSample[]>(`/api/host/history?range=${range}`),
  containers: () => get<Container[]>("/api/containers"),
  container: (id: string) => get<{ detail: Detail; live: Live | null }>(`/api/containers/${enc(id)}`),
  containerHistory: (id: string, range: Range) => get<CtrPoint[]>(`/api/containers/${enc(id)}/history?range=${range}`),
  containerNet: (id: string) => get<{ rx: number; tx: number; at: number }>(`/api/containers/${enc(id)}/net`),
  containerAction: (id: string, action: ContainerAction) => post<{ ok: true }>(`/api/containers/${enc(id)}/${action}`, undefined, 90_000),
  logsURL: (id: string, tail: number, follow: boolean) => `/api/containers/${enc(id)}/logs?tail=${tail}&follow=${follow ? 1 : 0}`,

  stacks: () => get<Stack[]>("/api/stacks"),
  stack: (project: string) => get<Stack>(`/api/stacks/${enc(project)}`),
  forgetStack: (project: string) => post<{ ok: true }>(`/api/stacks/${enc(project)}/forget`),

  storage: (refresh = false) => request<Storage>("GET", `/api/storage${refresh ? "?refresh=1" : ""}`, undefined, 90_000),
  removeImage: (id: string, name: string) => del<{ ok: true }>(`/api/images/${enc(id)}?name=${enc(name)}`),
  removeVolume: (name: string) => del<{ ok: true }>(`/api/volumes/${enc(name)}`),
  removeNetwork: (id: string, name: string) => del<{ ok: true }>(`/api/networks/${enc(id)}?name=${enc(name)}`),
  prune: (kind: PruneKind) => post<{ removed: number; reclaimed: number }>(`/api/prune/${kind}`, undefined, 600_000),

  activity: (q: ActivityQuery) => {
    const p = new URLSearchParams();
    for (const [k, v] of Object.entries(q)) if (v !== undefined && v !== "") p.set(k, String(v));
    return get<Activity[]>(`/api/activity?${p}`);
  },
  activityPage: (q: ActivityPageQuery) => {
    const p = new URLSearchParams();
    for (const [k, v] of Object.entries(q)) if (v !== undefined && v !== "") p.set(k, String(v));
    return get<ActivityPage>(`/api/activity/page?${p}`);
  },
  activityDetail: (id: number) => get<ActivityDetail>(`/api/activity/${id}`),
  sessions: () => get<SessionInfo[]>("/api/sessions"),
  revokeOthers: () => post<{ removed: number }>("/api/sessions/revoke-others"),

  settings: () => get<SettingsData>("/api/settings"),
  saveAlerts: (u: AlertsUpdate) => put<AlertsConfig>("/api/settings/alerts", u),
  testAlert: () => post<{ ok: true }>("/api/settings/alerts/test", undefined, 60_000),
  saveCleanup: (c: Pick<CleanupConfig, "enabled" | "weekday" | "hour" | "kinds">) => put<CleanupConfig>("/api/settings/cleanup", c),
  runCleanup: (kinds: CleanupKind[]) => post<{ kind: CleanupKind; removed: number; reclaimed: number; error?: string }[]>("/api/cleanup/run", { kinds }, 600_000),

  sites: () => get<Site[]>("/api/sites"),
  siteHistory: (host: string, range: Range) => get<SiteCheck[]>(`/api/sites/${enc(host)}/history?range=${range}`),
  addSite: (host: string) => post<{ ok: true }>("/api/sites", { host }),
  removeSite: (host: string) => del<{ ok: true }>(`/api/sites/${enc(host)}`),

  self: () => get<SelfStats>("/api/self"),
};

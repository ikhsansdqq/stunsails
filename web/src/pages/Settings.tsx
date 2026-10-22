import { useEffect, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { toast } from "sonner";
import { Bell, CalendarClock, Eraser, LogOut, Mail, Send, Webhook } from "@/components/ui/lucide";
import { api, type AlertsConfig, type CleanupConfig, type CleanupKind, type SmtpConfig } from "@/lib/api";
import { ago, formatBytes, formatDuration, formatFullDateTime, pct } from "@/lib/format";
import { PageHeader } from "@/components/Layout";
import { Block } from "@/components/ui/block";
import { Button } from "@/components/ui/button";
import { Section } from "@/components/ui/card";
import { Checkbox } from "@/components/ui/checkbox";
import { Chip } from "@/components/ui/chip";
import { Field, Input, PasswordInput } from "@/components/ui/field";
import { OptionSelect } from "@/components/ui/option-select";
import { Select } from "@/components/ui/select";
import { SimpleTable } from "@/components/ui/simple-table";
import { Stat, StatRow } from "@/components/ui/stat";
import { Switch } from "@/components/ui/switch";
import { Bar, Meter } from "@/components/charts";
import { Notice } from "@/components/Notice";
import { useRun } from "@/components/guard";
import { useConfirm } from "@/components/confirm";
import { useTheme } from "@/hooks/use-theme";

/** "Chrome on macOS" from a user agent, enough to tell sessions apart. */
function device(ua: string) {
  const browser = /Edg\//.test(ua) ? "Edge" : /Firefox\//.test(ua) ? "Firefox" : /Chrome\//.test(ua) ? "Chrome" : /Safari\//.test(ua) ? "Safari" : /curl/i.test(ua) ? "curl" : "A browser";
  const os = /iPhone|iPad/.test(ua) ? "iOS" : /Android/.test(ua) ? "Android" : /Mac OS X/.test(ua) ? "macOS" : /Windows/.test(ua) ? "Windows" : /Linux/.test(ua) ? "Linux" : "";
  return os ? `${browser} on ${os}` : browser;
}

const cmd = (s: string) => <code className="literal rounded bg-muted px-1.5 py-0.5 text-[0.8125rem]">{s}</code>;

export function SettingsPage() {
  const sessions = useQuery({ queryKey: ["sessions"], queryFn: api.sessions });
  const overview = useQuery({ queryKey: ["overview"], queryFn: api.overview, refetchInterval: 10_000 });
  const settings = useQuery({ queryKey: ["settings"], queryFn: api.settings });
  const run = useRun();
  const { theme, setTheme } = useTheme();
  const info = overview.data?.info;
  const others = (sessions.data ?? []).filter((s) => !s.current).length;
  return (
    <>
      <PageHeader title="Settings" sub="Alerts, cleanup, who is signed in, and what Harbor itself costs the server." />
      <div className="space-y-6">
        {settings.data && <Alerts initial={settings.data.alerts} />}
        {settings.data && <Cleanup initial={settings.data.cleanup} />}
        <Footprint />
        <Block
          title="Signed-in sessions"
          sub="A session lasts 7 days, or ends after 12 hours unused."
          actions={
            others > 0 && (
              <Button
                variant="outline"
                size="sm"
                icon={LogOut}
                onClick={() =>
                  void run(() => api.revokeOthers(), {
                    confirm: { title: "Sign out every other session?", description: `${others} other session${others === 1 ? "" : "s"} will need the password and a code again.`, confirmLabel: "Sign them out" },
                    success: (r) => `Signed out ${r.removed} other session${r.removed === 1 ? "" : "s"}`,
                    invalidate: [["sessions"]],
                  })
                }
              >
                Sign out the others
              </Button>
            )
          }
        >
          <SimpleTable
            label="Sessions"
            rows={sessions.data ?? []}
            rowKey={(s) => s.id}
            empty="Loading…"
            columns={[
              {
                header: "Device",
                cell: (s) => (
                  <span className="flex items-center gap-2" title={s.ua}>
                    {device(s.ua)} {s.current && <Chip tone="good">this one</Chip>}
                  </span>
                ),
              },
              { header: "Address", cell: (s) => <span className="num">{s.ip}</span> },
              { header: "Signed in", cell: (s) => <span title={formatFullDateTime(s.created * 1000)}>{ago(s.created)}</span> },
              { header: "Last seen", cell: (s) => ago(s.seen) },
            ]}
          />
        </Block>
        <Section title="Sign-in" sub="A password, then a code from Google Authenticator. Changes need a fresh code every five minutes.">
          <ul className="space-y-2 text-sm">
            <li>Change the password: {cmd("sudo server-monitor passwd")}, then {cmd("sudo systemctl restart server-monitor")}.</li>
            <li>Pair a new phone (the old one stops working): {cmd("sudo server-monitor enroll")}, then restart.</li>
            <li>Add a second phone with the same key: {cmd("sudo server-monitor enroll --show")}.</li>
            <li>Five wrong tries from one address lock it out for 15 minutes. Every attempt is in Activity.</li>
          </ul>
          {settings.data?.auth_log_error && (
            <p className="mt-3 text-sm text-warning">Server sign-ins can't be read: {settings.data.auth_log_error}. Reinstall with the deploy script, which adds Harbor to the adm group.</p>
          )}
        </Section>
        <Section title="Appearance" sub="Kept in this browser.">
          <div className="flex flex-wrap gap-x-8 gap-y-4">
            <div>
              <p className="mb-1.5 text-sm font-medium">Theme</p>
              <OptionSelect
                label="Theme"
                value={theme}
                onChange={(t) => {
                  setTheme(t);
                  toast(t === "system" ? "Following the system theme" : t === "dark" ? "Dark mode on" : "Light mode on");
                }}
                options={[
                  { id: "system", label: "System" },
                  { id: "light", label: "Light" },
                  { id: "dark", label: "Dark" },
                ]}
              />
            </div>
          </div>
        </Section>
        {info && (
          <Section title="This server">
            <dl className="grid gap-x-6 gap-y-3 text-sm sm:grid-cols-2 lg:grid-cols-3">
              {(
                [
                  ["Hostname", info.hostname],
                  ["System", info.os],
                  ["Kernel", info.kernel],
                  ["CPU", `${info.cpus} × ${info.cpu_model || "cores"}`],
                  ["Up for", formatDuration(info.uptime_sec)],
                  ["Load", info.load.map((l) => l.toFixed(2)).join(" · ")],
                  ["Docker", overview.data ? `${overview.data.docker.version} (API ${overview.data.docker.api_version})` : ""],
                ] as const
              ).map(([k, v]) => (
                <div key={k} className="min-w-0">
                  <dt className="text-[0.8125rem] text-muted-foreground">{k}</dt>
                  <dd className="mt-0.5 break-words">{v || "—"}</dd>
                </div>
              ))}
            </dl>
            <h3 className="mt-5 mb-2 text-[0.8125rem] text-muted-foreground">Disks</h3>
            <div className="grid gap-4 sm:grid-cols-2">
              {info.disks.map((d) => (
                <div key={d.mount}>
                  <p className="flex justify-between text-sm">
                    <span className="font-medium">{d.mount}</span>
                    <span className="num text-muted-foreground">
                      {formatBytes(d.used)} of {formatBytes(d.total)}
                    </span>
                  </p>
                  <Bar percent={pct(d.used, d.total)} label={`${d.mount} used`} className="mt-2" />
                  <p className="mt-1 text-xs text-muted-foreground">{formatBytes(d.free)} free</p>
                </div>
              ))}
            </div>
          </Section>
        )}
      </div>
    </>
  );
}

function Alerts({ initial }: { initial: AlertsConfig }) {
  const run = useRun();
  const confirm = useConfirm();
  const [smtp, setSmtp] = useState<SmtpConfig>({ ...initial.smtp, password: "" });
  const [to, setTo] = useState(initial.smtp.to.join(", "));
  const [hookOn, setHookOn] = useState(initial.webhook.enabled);
  const [hookUrl, setHookUrl] = useState("");
  const [rules, setRules] = useState(initial.rules);
  const [cfg, setCfg] = useState(initial);
  useEffect(() => setCfg(initial), [initial]);
  const set = (k: keyof SmtpConfig, v: string | number | boolean) => setSmtp((s) => ({ ...s, [k]: v }));

  const save = (extra: { clear_password?: boolean; clear_webhook?: boolean } = {}) =>
    run(
      () =>
        api.saveAlerts({
          smtp: { ...smtp, port: Number(smtp.port) || 587, to: to.split(/[,;\s]+/).filter(Boolean) },
          webhook: { enabled: extra.clear_webhook ? false : hookOn, url: hookUrl || undefined },
          rules,
          ...extra,
        }),
      { success: "Alert settings saved", invalidate: [["settings"]] },
    ).then((r) => {
      if (r) {
        setCfg(r);
        setHookUrl("");
        setSmtp((s) => ({ ...s, password: "" }));
        if (extra.clear_webhook) setHookOn(false);
      }
      return r;
    });

  const anyOn = cfg.smtp.enabled || cfg.webhook.enabled;
  return (
    <Section
      title="Alerts"
      sub="Harbor tells you when something breaks, once, and again when it's fixed. The same problem stays quiet for 30 minutes."
      actions={
        <Button size="sm" variant="outline" icon={Send} disabled={!anyOn} onClick={() => void run(() => api.testAlert(), { success: "Test alert sent. Check your inbox or channel." })}>
          Send a test
        </Button>
      }
    >
      {!anyOn && (
        <div className="mb-4">
          <Notice tone="amber" title="No alert channel is on">
            Turn on email or a webhook below, save, then send a test.
          </Notice>
        </div>
      )}
      {cfg.active.length > 0 && (
        <div className="mb-4 rounded-md border border-critical/30 bg-critical/5 px-3 py-2.5 text-sm">
          <p className="font-medium text-critical">Alerting now</p>
          <ul className="mt-1 space-y-0.5 text-[0.8125rem]">
            {cfg.active.map((a) => (
              <li key={a.rule + a.target}>
                {a.title} <span className="text-muted-foreground">since {ago(a.since)}</span>
              </li>
            ))}
          </ul>
        </div>
      )}
      <div className="grid gap-6 lg:grid-cols-2">
        <fieldset className="space-y-3">
          <legend className="mb-2 flex w-full items-center justify-between gap-2 text-sm font-medium">
            <span className="flex items-center gap-2">
              <Mail size={16} className="text-muted-foreground" aria-hidden /> Email
            </span>
            <Switch checked={smtp.enabled} onChange={(on) => set("enabled", on)} label="Send alerts by email" />
          </legend>
          <div className="grid grid-cols-[1fr_6rem] gap-3">
            <Field label="SMTP server" htmlFor="smtp-host">
              <Input id="smtp-host" placeholder="smtp.gmail.com" value={smtp.host} onChange={(e) => set("host", e.target.value)} />
            </Field>
            <Field label="Port" htmlFor="smtp-port">
              <Input id="smtp-port" inputMode="numeric" placeholder="587" value={smtp.port || ""} onChange={(e) => set("port", Number(e.target.value.replace(/\D/g, "")))} />
            </Field>
          </div>
          <div className="grid gap-3 sm:grid-cols-2">
            <Field label="Username" htmlFor="smtp-user">
              <Input id="smtp-user" autoComplete="off" placeholder="you@gmail.com" value={smtp.user} onChange={(e) => set("user", e.target.value)} />
            </Field>
            <Field label="Password" htmlFor="smtp-pass" hint={cfg.smtp.password_set ? "Saved. Leave empty to keep it." : "For Gmail, an app password."}>
              <PasswordInput id="smtp-pass" autoComplete="new-password" placeholder={cfg.smtp.password_set ? "••••••••" : "App password"} value={smtp.password ?? ""} onChange={(e) => set("password", e.target.value)} />
            </Field>
          </div>
          <div className="grid gap-3 sm:grid-cols-2">
            <Field label="From" htmlFor="smtp-from">
              <Input id="smtp-from" placeholder="harbor@yourdomain.com" value={smtp.from} onChange={(e) => set("from", e.target.value)} />
            </Field>
            <Field label="To" htmlFor="smtp-to" hint="Several? Separate with commas.">
              <Input id="smtp-to" placeholder="you@yourdomain.com" value={to} onChange={(e) => setTo(e.target.value)} />
            </Field>
          </div>
          {cfg.smtp.password_set && (
            <button
              type="button"
              className="text-[0.8125rem] text-muted-foreground underline-offset-4 hover:text-critical hover:underline"
              onClick={async () => {
                if (await confirm({ title: "Forget the saved SMTP password?", description: "Email alerts stop working until you enter it again.", confirmLabel: "Forget it" })) void save({ clear_password: true });
              }}
            >
              Forget the saved password
            </button>
          )}
        </fieldset>
        <fieldset className="space-y-3">
          <legend className="mb-2 flex w-full items-center justify-between gap-2 text-sm font-medium">
            <span className="flex items-center gap-2">
              <Webhook size={16} className="text-muted-foreground" aria-hidden /> Webhook
            </span>
            <Switch checked={hookOn} onChange={setHookOn} label="Send alerts to a webhook" />
          </legend>
          <Field
            label="URL"
            htmlFor="hook-url"
            hint={cfg.webhook.url_set ? `Saved (${cfg.webhook.host}). Leave empty to keep it.` : "Discord, Slack, ntfy or n8n: Harbor posts JSON with content and text fields they read as is."}
          >
            <Input id="hook-url" type="url" placeholder={cfg.webhook.url_set ? `https://${cfg.webhook.host}/…` : "https://discord.com/api/webhooks/…"} value={hookUrl} onChange={(e) => setHookUrl(e.target.value)} />
          </Field>
          {cfg.webhook.url_set && (
            <button
              type="button"
              className="text-[0.8125rem] text-muted-foreground underline-offset-4 hover:text-critical hover:underline"
              onClick={async () => {
                if (await confirm({ title: "Remove the webhook?", description: `Alerts stop going to ${cfg.webhook.host}.`, confirmLabel: "Remove webhook" })) void save({ clear_webhook: true });
              }}
            >
              Remove the webhook
            </button>
          )}
          <h3 className="flex items-center gap-2 pt-3 text-sm font-medium">
            <Bell size={16} className="text-muted-foreground" aria-hidden /> Tell me when
          </h3>
          <ul className="space-y-1.5">
            {cfg.rule_list.map((r) => (
              <li key={r.ID}>
                <label className="flex items-center gap-2.5 text-sm">
                  <Checkbox checked={!!rules[r.ID]} onChange={(e) => setRules((x) => ({ ...x, [r.ID]: e.target.checked }))} />
                  {r.Label}
                </label>
              </li>
            ))}
          </ul>
        </fieldset>
      </div>
      <div className="mt-5 flex justify-end">
        <Button onClick={() => void save()}>Save alert settings</Button>
      </div>
    </Section>
  );
}

const days = ["Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"];
const kinds: { id: CleanupKind; label: string; hint: string }[] = [
  { id: "build-cache", label: "Build cache", hint: "Rebuilds start from scratch once." },
  { id: "dangling", label: "Untagged images", hint: "Leftovers from rebuilds." },
  { id: "containers", label: "Stopped containers", hint: "Compose recreates its own." },
  { id: "networks", label: "Unused networks", hint: "Compose recreates its own." },
];

function Cleanup({ initial }: { initial: CleanupConfig }) {
  const run = useRun();
  const [c, setC] = useState(initial);
  useEffect(() => setC(initial), [initial]);
  const toggle = (k: CleanupKind, on: boolean) => setC((x) => ({ ...x, kinds: on ? [...x.kinds, k] : x.kinds.filter((y) => y !== k) }));
  const what = c.kinds.map((k) => kinds.find((x) => x.id === k)?.label.toLowerCase()).join(", ");
  return (
    <Section
      title="Scheduled cleanup"
      sub="Frees what Docker no longer needs, once a week. Volumes are never touched."
      actions={
        <Button
          size="sm"
          variant="outline"
          icon={Eraser}
          disabled={c.kinds.length === 0}
          onClick={() =>
            void run(() => api.runCleanup(c.kinds), {
              confirm: { title: "Clean up now?", description: `Deletes ${what}. It can't be undone, though everything is rebuilt or recreated when needed.`, confirmLabel: "Clean up now" },
              success: (r) => `Freed ${formatBytes(r.reduce((n, x) => n + x.reclaimed, 0))}, removed ${r.reduce((n, x) => n + x.removed, 0)} items`,
              invalidate: [["settings"], ["storage"], ["overview"]],
            })
          }
        >
          Clean up now
        </Button>
      }
    >
      <div className="grid gap-6 lg:grid-cols-2">
        <div className="space-y-3">
          <label className="flex items-center justify-between gap-2 text-sm font-medium">
            <span className="flex items-center gap-2">
              <CalendarClock size={16} className="text-muted-foreground" aria-hidden /> Every week
            </span>
            <Switch checked={c.enabled} onChange={(on) => setC({ ...c, enabled: on })} label="Run the cleanup every week" />
          </label>
          <div className="grid grid-cols-2 gap-3">
            <Field label="Day" htmlFor="clean-day">
              <Select id="clean-day" value={String(c.weekday)} onChange={(e) => setC({ ...c, weekday: Number(e.target.value) })}>
                {days.map((d, i) => (
                  <option key={d} value={String(i)}>
                    {d}
                  </option>
                ))}
              </Select>
            </Field>
            <Field label="Hour (server time)" htmlFor="clean-hour">
              <Select id="clean-hour" value={String(c.hour)} onChange={(e) => setC({ ...c, hour: Number(e.target.value) })}>
                {Array.from({ length: 24 }, (_, h) => (
                  <option key={h} value={String(h)}>
                    {String(h).padStart(2, "0")}:00
                  </option>
                ))}
              </Select>
            </Field>
          </div>
          <p className="text-[0.8125rem] text-muted-foreground">
            {c.last_run ? `Last run ${ago(c.last_run)}: ${c.last_result}.` : "Hasn't run yet."} freelance-dashboard's own housekeeping timer may also prune; the two don't conflict.
          </p>
        </div>
        <ul className="space-y-2">
          {kinds.map((k) => (
            <li key={k.id}>
              <label className="flex items-start gap-2.5 text-sm">
                <Checkbox className="mt-0.5" checked={c.kinds.includes(k.id)} onChange={(e) => toggle(k.id, e.target.checked)} />
                <span>
                  {k.label} <span className="text-muted-foreground">· {k.hint}</span>
                </span>
              </label>
            </li>
          ))}
        </ul>
      </div>
      <div className="mt-5 flex justify-end">
        <Button
          onClick={() =>
            void run(() => api.saveCleanup(c), {
              confirm: c.enabled
                ? { title: `Clean up every ${days[c.weekday]} at ${String(c.hour).padStart(2, "0")}:00?`, description: `Harbor will delete ${what || "nothing"} each week without asking.`, confirmLabel: "Save schedule", tone: "default" }
                : undefined,
              success: c.enabled ? "Cleanup scheduled" : "Weekly cleanup off",
              invalidate: [["settings"]],
            })
          }
        >
          Save schedule
        </Button>
      </div>
    </Section>
  );
}

/** What Harbor itself costs the server. */
function Footprint() {
  const self = useQuery({ queryKey: ["self"], queryFn: api.self, refetchInterval: 10_000 });
  const s = self.data;
  if (!s) return null;
  const mem = s.mem_current ?? s.rss;
  const max = s.mem_max && s.mem_max > 0 ? s.mem_max : 0;
  return (
    <Section title="Harbor itself" sub={`What watching this server costs it. Up ${formatDuration(s.uptime_sec)}.`}>
      <StatRow>
        <Meter label="Memory" value={formatBytes(mem)} percent={max ? pct(mem, max) : 0} hint={max ? `of its ${formatBytes(max)} limit; peak ${formatBytes(s.mem_peak ?? s.rss_peak)}` : `peak ${formatBytes(s.rss_peak)}`} />
        <Stat label="CPU" value={s.cpu_pct !== undefined ? `${s.cpu_pct.toFixed(2)}%` : "—"} note={`of one core; ${(s.cpu_sec ?? 0).toFixed(1)} s in total`} />
        <Stat label="On disk" value={formatBytes((s.db_bytes ?? 0) + (s.binary_bytes ?? 0))} note={`${formatBytes(s.db_bytes ?? 0)} history, ${formatBytes(s.binary_bytes ?? 0)} program`} />
        <Stat label="Compose helper" value={s.helper_image_bytes ? formatBytes(s.helper_image_bytes) : "—"} note={`${s.helper_image ?? "docker CLI"} image, used only while a stack action runs`} />
      </StatRow>
      <p className="mt-3 text-xs text-muted-foreground">
        {s.goroutines} goroutines, {s.threads} threads, {formatBytes(s.go_heap)} Go heap. Docker answers one container list every 10 seconds and streams its events.
      </p>
    </Section>
  );
}

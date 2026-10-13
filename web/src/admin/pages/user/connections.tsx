import { ChevronDown } from "lucide-react";
import { useMemo, useState } from "react";
import { errorText, type Schemas, type User } from "../../../api/client";
import { useUserConnections } from "../../../api/hooks";
import { Drawer } from "../../../components/overlay";
import { Avatar, ErrorState, Skeleton } from "../../../components/ui";
import { t } from "../../../i18n";
import { classify, parseWatch, splitDest, type Flag, type ServiceKind, type Target } from "../../../lib/apps";
import { bytes, dateShort, time } from "../../../lib/format";

type Conn = Schemas["ConnectionView"];

/** A log line with what the window shows of it worked out once. */
type Line = {
  id: string;
  c: Conn;
  /** The site's name when it is known, else the address. */
  host: string;
  addr: string;
  port: number;
  target: Target;
};

type Group = {
  target: Target;
  lines: Line[];
  up: number;
  down: number;
  open: number;
  last: string;
  hosts: Map<string, { n: number; bytes: number }>;
};

const SERVICE_NAMES = {
  dns: "userDrawer.connSvcDns",
  push: "userDrawer.connSvcPush",
  check: "userDrawer.connSvcCheck",
  ads: "userDrawer.connSvcAds",
} as const satisfies Record<ServiceKind, string>;

const FLAG_NAMES = {
  adult: "userDrawer.connFlagAdult",
  tor: "userDrawer.connFlagTor",
  watch: "userDrawer.connFlagWatch",
} as const satisfies Record<Flag, string>;

function FlagBadge({ flag }: { flag?: Flag }) {
  if (!flag) return null;
  return <span className="ml-2 rounded-[6px] bg-[var(--bad)] px-1.5 py-0.5 text-[11px] font-medium text-white">{t(FLAG_NAMES[flag])}</span>;
}

// The admin's own list lives in this browser: it is a way to look, not a setting of the panel.
const WATCH_KEY = "mirai.connWatch";

function loadWatch(): string {
  try {
    return localStorage.getItem(WATCH_KEY) ?? "";
  } catch {
    return "";
  }
}

function saveWatch(text: string) {
  try {
    localStorage.setItem(WATCH_KEY, text);
  } catch {
    // A browser without storage keeps the list until the page is closed.
  }
}

function targetName(x: Target): string {
  if (x.service) return t(SERVICE_NAMES[x.service]);
  return x.kind === "ip" ? t("userDrawer.connUnnamed") : x.name;
}

/** The standard port of a site says nothing; any other one is worth showing. */
function hostPort(l: Line): string {
  return l.port === 443 || l.port === 80 || l.port === 0 ? l.host : `${l.host}:${l.port}`;
}

function seedOf(s: string): number {
  let n = 0;
  for (let i = 0; i < s.length; i++) n = (n * 31 + s.charCodeAt(i)) % 997;
  return n;
}

function toLines(conns: Conn[], watch: readonly string[]): Line[] {
  const seen = new Map<string, number>();
  return conns.map((c) => {
    const { host: addr, port } = splitDest(c.dest);
    const base = `${c.node_id}|${c.network}|${c.ip}|${c.dest}|${c.started_at}`;
    const n = seen.get(base) ?? 0;
    seen.set(base, n + 1);
    return { id: `${base}|${n}`, c, host: c.host || addr, addr, port, target: classify(c.host || addr, addr, port, watch) };
  });
}

function toGroups(lines: Line[]): Group[] {
  const groups = new Map<string, Group>();
  for (const l of lines) {
    let g = groups.get(l.target.key);
    if (!g) {
      g = { target: l.target, lines: [], up: 0, down: 0, open: 0, last: l.c.started_at, hosts: new Map() };
      groups.set(l.target.key, g);
    }
    g.lines.push(l);
    g.up += l.c.up;
    g.down += l.c.down;
    if (!l.c.ended_at) g.open++;
    const at = l.c.ended_at ?? l.c.started_at;
    if (at > g.last) g.last = at;
    const h = g.hosts.get(l.host) ?? { n: 0, bytes: 0 };
    h.n++;
    h.bytes += l.c.up + l.c.down;
    g.hosts.set(l.host, h);
  }
  // Flagged first, then by volume: the order holds still while the log refreshes, unlike
  // "the latest first".
  return [...groups.values()].sort((a, b) => Number(!!b.target.flag) - Number(!!a.target.flag) || b.up + b.down - (a.up + a.down) || a.target.key.localeCompare(b.target.key));
}

/** The user's connection log: which apps and sites the traffic goes to. */
export function ConnectionsDialog({ u, open, onOpenChange }: { u: User; open: boolean; onOpenChange: (v: boolean) => void }) {
  const log = useUserConnections(u.id, open);
  const [view, setView] = useState<"apps" | "all">("apps");
  const [filter, setFilter] = useState("");
  const [service, setService] = useState(false);
  const [flagged, setFlagged] = useState(false);
  const [watchText, setWatchText] = useState(loadWatch);
  const watch = useMemo(() => parseWatch(watchText), [watchText]);
  const [expanded, setExpanded] = useState<ReadonlySet<string>>(new Set());

  const conns = log.data?.conns;
  const all = useMemo(() => toLines(conns ?? [], watch), [conns, watch]);
  const flaggedCount = useMemo(() => all.filter((l) => l.target.flag).length, [all]);
  const serviceCount = useMemo(() => all.filter((l) => l.target.kind === "service").length, [all]);
  const list = useMemo(() => {
    const q = filter.trim().toLowerCase();
    return all.filter((l) => {
      if (flagged && !l.target.flag) return false;
      if (!service && l.target.kind === "service") return false;
      return !q || [l.host, l.c.dest, l.c.ip, l.c.inbound, l.c.node, targetName(l.target)].some((v) => v.toLowerCase().includes(q));
    });
  }, [all, filter, service, flagged]);
  const groups = useMemo(() => toGroups(list), [list]);
  const live = all.filter((l) => !l.c.ended_at).length;
  const down = log.data?.unavailable ?? [];
  const toggle = (key: string) =>
    setExpanded((prev) => {
      const next = new Set(prev);
      if (!next.delete(key)) next.add(key);
      return next;
    });

  return (
    <Drawer presentation="modal" modalWidth="protocol" open={open} onOpenChange={onOpenChange} title={t("userDrawer.connectionsTitle", { name: u.name })} meta={log.data ? <span>{t("userDrawer.connectionsCount", { open: live, total: all.length })}</span> : undefined}>
      <div className="flex flex-col gap-3 py-4">
        <div className="flex flex-wrap items-center gap-2">
          <div className="seg" role="group" aria-label={t("userDrawer.connections")}>
            <button type="button" aria-pressed={view === "apps"} onClick={() => setView("apps")}>
              {t("userDrawer.connViewApps")}
            </button>
            <button type="button" aria-pressed={view === "all"} onClick={() => setView("all")}>
              {t("userDrawer.connViewAll")}
            </button>
          </div>
          <label className="flex items-center gap-2 text-[13px] text-[var(--ink-600)]">
            <input type="checkbox" checked={service} onChange={(e) => setService(e.target.checked)} />
            {t("userDrawer.connService", { n: serviceCount })}
          </label>
          <label className="flex items-center gap-2 text-[13px] text-[var(--ink-600)]">
            <input type="checkbox" checked={flagged} onChange={(e) => setFlagged(e.target.checked)} />
            {t("userDrawer.connOnlyFlagged", { n: flaggedCount })}
          </label>
        </div>
        <details className="text-[13px]">
          <summary className="cursor-pointer text-[var(--ink-600)]">{t("userDrawer.connWatchTitle", { n: watch.length })}</summary>
          <textarea
            className="input mono mt-2 min-h-20 py-2"
            style={{ height: "auto" }}
            value={watchText}
            onChange={(e) => {
              setWatchText(e.target.value);
              saveWatch(e.target.value);
            }}
            placeholder="example.com, shop.example.org"
            aria-label={t("userDrawer.connWatchTitle", { n: watch.length })}
            spellCheck={false}
          />
          <p className="mt-1 text-xs text-[var(--ink-500)]">{t("userDrawer.connWatchHint")}</p>
        </details>
        <input className="input" value={filter} onChange={(e) => setFilter(e.target.value)} placeholder={t("userDrawer.connectionsFilter")} aria-label={t("userDrawer.connectionsFilter")} spellCheck={false} />
        {down.length > 0 ? <p className="text-xs text-[var(--bad)]">{t("userDrawer.connectionsUnavailable", { nodes: down.join(", ") })}</p> : null}
        {log.isPending ? (
          <Skeleton style={{ height: 160, borderRadius: 16 }} />
        ) : log.data === undefined ? (
          <ErrorState text={errorText(log.error)} onRetry={() => void log.refetch()} />
        ) : list.length === 0 ? (
          <p className="text-[13px] text-[var(--ink-500)]">{all.length === 0 ? t("userDrawer.connectionsEmpty") : t("userDrawer.connectionsNoMatch")}</p>
        ) : view === "apps" ? (
          <ul className="flex flex-col gap-1.5">
            {groups.map((g) => (
              <GroupRow key={g.target.key} g={g} open={expanded.has(g.target.key)} onToggle={() => toggle(g.target.key)} />
            ))}
          </ul>
        ) : (
          <ul className="flex flex-col gap-1.5">
            {list.slice(0, SHOWN).map((l) => (
              <LineRow key={l.id} l={l} named />
            ))}
          </ul>
        )}
        <p className="text-xs text-[var(--ink-500)]">{t("userDrawer.connectionsNote")}</p>
      </div>
    </Drawer>
  );
}

/** How many lines one list draws: the window refreshes every few seconds. */
const SHOWN = 200;
const GROUP_LINES = 30;
const GROUP_HOSTS = 12;

function GroupRow({ g, open, onToggle }: { g: Group; open: boolean; onToggle: () => void }) {
  const name = targetName(g.target);
  const hosts = [...g.hosts.entries()].sort((a, b) => b[1].bytes - a[1].bytes);
  // A site's own name is its only host: nothing to add under the title.
  const preview = hosts.filter(([h]) => h !== name).slice(0, 2).map(([h]) => h).join(", ");
  return (
    <li className="panel-soft">
      <button type="button" className="grid w-full grid-cols-[36px_minmax(0,1fr)_auto_16px] items-center gap-3 px-3 py-2 text-left" aria-expanded={open} onClick={onToggle}>
        <Avatar name={name} seed={seedOf(g.target.key)} size="sm" />
        <div className="min-w-0">
          <div className="truncate text-[13px] font-medium text-[var(--ink-900)]">
            {name}
            <FlagBadge flag={g.target.flag} />
            {g.open > 0 ? <span className="ml-2 text-xs font-normal text-[var(--leaf-700)]">{t("userDrawer.connOpenCount", { n: g.open })}</span> : null}
          </div>
          <div className="truncate text-xs text-[var(--ink-500)]">
            {t("userDrawer.connGroupMeta", { n: g.lines.length })}
            {preview ? ` · ${preview}` : ""}
          </div>
        </div>
        <div className="text-right text-xs">
          <div className="num text-[var(--ink-700)]">
            ↑ {bytes(g.up)} · ↓ {bytes(g.down)}
          </div>
          <div className="text-[var(--ink-500)]">
            {dateShort(g.last)} {time(g.last)}
          </div>
        </div>
        <ChevronDown size={16} aria-hidden className={open ? "rotate-180 text-[var(--ink-500)]" : "text-[var(--ink-500)]"} />
      </button>
      {open ? (
        <div className="flex flex-col gap-2 border-t border-[var(--hairline)] px-3 py-2">
          {hosts.length > 1 ? (
            <ul className="flex flex-col gap-0.5 text-xs">
              {hosts.slice(0, GROUP_HOSTS).map(([h, s]) => (
                <li key={h} className="grid grid-cols-[minmax(0,1fr)_auto_auto] gap-3">
                  <span className="mono truncate" title={h}>
                    {h}
                  </span>
                  <span className="num text-[var(--ink-500)]">×{s.n}</span>
                  <span className="num w-[72px] text-right text-[var(--ink-600)]">{bytes(s.bytes)}</span>
                </li>
              ))}
              {hosts.length > GROUP_HOSTS ? <li className="text-[var(--ink-500)]">{t("userDrawer.connMore", { n: hosts.length - GROUP_HOSTS })}</li> : null}
            </ul>
          ) : null}
          <ul className="flex flex-col gap-1.5">
            {g.lines.slice(0, GROUP_LINES).map((l) => (
              <LineRow key={l.id} l={l} />
            ))}
          </ul>
          {g.lines.length > GROUP_LINES ? <p className="text-xs text-[var(--ink-500)]">{t("userDrawer.connMore", { n: g.lines.length - GROUP_LINES })}</p> : null}
        </div>
      ) : null}
    </li>
  );
}

/** One connection; `named` puts the app's name in front, for the list that is not grouped. */
function LineRow({ l, named }: { l: Line; named?: boolean }) {
  const c = l.c;
  const app = named && l.target.kind !== "site" ? targetName(l.target) : "";
  return (
    <li className="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-x-3 gap-y-0.5 rounded-[10px] bg-[var(--hover)] px-3 py-1.5 text-xs">
      <div className="truncate text-[13px] text-[var(--ink-900)]" title={c.dest}>
        {app ? <span className="font-medium">{app} · </span> : null}
        <span className="mono">{hostPort(l)}</span>
        {named ? <FlagBadge flag={l.target.flag} /> : null}
      </div>
      <div className="num text-right text-[var(--ink-600)]">
        ↑ {bytes(c.up)} · ↓ {bytes(c.down)}
      </div>
      <div className="truncate text-[var(--ink-500)]">
        <span className="uppercase">{c.network}</span>
        {l.host !== l.addr ? <span className="mono"> · {l.addr}</span> : null} · {t("userDrawer.connFrom", { ip: c.ip })} · {c.inbound} · {c.node}
      </div>
      <div className="text-right text-[var(--ink-500)]">
        {dateShort(c.started_at)} {time(c.started_at)}
        {" — "}
        {c.ended_at ? time(c.ended_at) : <span className="text-[var(--leaf-700)]">{t("userDrawer.connectionsOpen")}</span>}
      </div>
    </li>
  );
}

import { Select } from "../../components/select";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { AnimatePresence, motion, Reorder, useDragControls, useReducedMotion } from "motion/react";
import { Link, useBlocker, useNavigate, useSearch } from "@tanstack/react-router";
import { UserRound, Smartphone, LifeBuoy, Wallet, ArrowUpRight, GripVertical, Bell, Bot, Globe, LayoutList, Link2, Megaphone, Network, Plus, PlugZap, Send, Shield, Trash2, TriangleAlert } from "lucide-react";
import { createElement, useMemo, useState, type Dispatch, type FormEvent, type SetStateAction, type ReactNode } from "react";
import { api, ApiError, errorText, unwrap, type Schemas } from "../../api/client";
import { qk, useNodes, useSettings } from "../../api/hooks";
import { useDraft } from "../../lib/draft";
import { TELEGRAM_TABS } from "../search";
import { ago, num } from "../../lib/format";
import { Confirm } from "../../components/overlay";
import { QueryBoundary } from "../../components/query";
import { Columns, Tabs } from "../../components/tabs";
import { useToast } from "../../components/toast";
import { Bar, Button, Field, PageHeader, Pill, Segmented, Skeleton } from "../../components/ui";
import { Switch } from "../../components/switch";
import { t, tMaybe, useLocale } from "../../i18n";

type View = Schemas["TelegramView"];

function TelegramText({ text }: { text: string }) {
  const root: { tag: string; children: ReactNode[] } = { tag: "", children: [] };
  const stack = [root];
  const parts = text.split(/(<\/?(?:b|i|u|s|code)>)/g);
  for (const [index, part] of parts.entries()) {
    const tag = /^<(\/)?(b|i|u|s|code)>$/.exec(part);
    if (!tag) { stack[stack.length - 1]!.children.push(part); continue; }
    if (!tag[1]) { stack.push({ tag: tag[2]!, children: [] }); continue; }
    const element = stack.pop();
    if (!element || stack.length === 0 || element.tag !== tag[2]) return <>{text}</>;
    stack[stack.length - 1]!.children.push(createElement(element.tag, { key: index }, ...element.children));
  }
  return <>{stack.length === 1 ? root.children : text}</>;
}
type Config = Schemas["Config"];
type MenuButton = Schemas["MenuButton"];
type TextKey = keyof Schemas["Texts"];

const TAB_ICONS = { connect: PlugZap, menu: LayoutList, notify: Bell, infra: Network, admin: Shield, broadcast: Megaphone } as const;

function useTelegram() {
  return useQuery({
    queryKey: qk.telegram,
    queryFn: ({ signal }) => unwrap(api.GET("/api/v1/telegram", { signal })),
    // A broadcast in progress moves every second; otherwise little changes.
    refetchInterval: (q) => (q.state.data?.broadcast?.active ? 2_000 : 10_000),
  });
}

function usePatchTelegram() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: Schemas["PatchTelegramInputBody"]) => {
      if(body.config) body={...body,config:{...body.config,admin:{...body.config.admin,buttons:body.config.admin.buttons.map(({id,action,label,on,row})=>({id,action,label,on,row}))}}};
      return unwrap(api.PATCH("/api/v1/telegram",{body}));
    },
    onSuccess: (v) => qc.setQueryData(qk.telegram, v),
  });
}

/**
 * The bot in four sections: connecting it, its menu and texts, notifications and options,
 * broadcasts. Menu, texts and options are one draft saved together from the bar below,
 * whichever section they were changed in.
 */
export function TelegramPage() {
  const tg = useTelegram();
  return (
    <>
      <PageHeader title={t("nav.telegram")} sub={t("telegram.subtitle")} />
      <QueryBoundary
        query={tg}
        pending={
          <div className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_360px]">
            <Skeleton style={{ height: 320, borderRadius: 20 }} />
            <Skeleton style={{ height: 420, borderRadius: 20 }} />
          </div>
        }
        wrap={(state) => <section className="card glass">{state}</section>}
      >
        {(v) => <TelegramBody v={v} />}
      </QueryBoundary>
    </>
  );
}

function TelegramBody({ v }: { v: View }) {
  const { tab } = useSearch({ from: "/_app/telegram" });
  const navigate = useNavigate({ from: "/telegram" });
  const patch = usePatchTelegram();
  const toast = useToast();
  // The draft follows the server's copy while untouched and keeps the edits when the copy
  // changes under it (a poll, the bot saved from another session).
  const { draft, setDraft, dirty, reset } = useDraft(v.config);
  const infrastructure = useDraft(v.infrastructure);
  const save = () =>
    patch.mutate(
      { config: draft },
      {
        onSuccess: (r) => {
          setDraft(r.config);
          toast.ok(t("telegram.saved"));
        },
        onError: (e) => toast.error(errorText(e)),
      },
    );
  // Leaving the page drops the draft: ask first. Switching the section stays on the page.
  const leave = useBlocker({ shouldBlockFn: ({ current, next }) => (dirty || infrastructure.dirty) && current.pathname !== next.pathname, enableBeforeUnload: () => dirty || infrastructure.dirty, withResolver: true });

  return (
    <>
      <Tabs
        id="telegram"
        label={t("telegram.sections")}
        tabs={TELEGRAM_TABS.map((id) => ({ id, label: t(`telegram.tabs.${id}`), icon: TAB_ICONS[id] }))}
        value={tab}
        onChange={(next) => void navigate({ search: { tab: next }, replace: true })}
      >
        {tab === "connect" ? (
          <Columns
            wide="left"
            left={
              <>
                <ConnectCard v={v} />
                <RouteCard v={v} />
              </>
            }
            right={<Preview draft={draft} v={v} />}
          />
        ) : tab === "menu" ? (
          <Columns
            wide="left"
            left={
              <>
                <MenuCard draft={draft} setDraft={setDraft} />
                <TextsCard draft={draft} setDraft={setDraft} defaults={v.defaults} />
              </>
            }
            right={<Preview draft={draft} v={v} />}
          />
        ) : tab === "notify" ? (
          <Columns wide="left" left={<OptionsCard draft={draft} setDraft={setDraft} v={v} />} right={<Preview draft={draft} v={v} />} />
        ) : tab === "infra" ? (
          <div className="grid w-full items-start gap-4 xl:grid-cols-2">
            <InfrastructureCard v={v} draft={infrastructure.draft} setDraft={infrastructure.setDraft} dirty={infrastructure.dirty} />
            <BackupCard />
          </div>
        ) : tab === "admin" ? (
          <><AdminMenuCard draft={draft} setDraft={setDraft} /><DeviceResetCard v={v} /></>
        ) : (
          <div className="w-full">
            <BroadcastCard v={v} />
          </div>
        )}
      </Tabs>
      <AnimatePresence>
        {dirty ? (
          <motion.div
            className="bulk-bar glass-strong"
            role="region"
            aria-label={t("telegram.unsaved")}
            initial={{ opacity: 0, y: 24, x: "-50%" }}
            animate={{ opacity: 1, y: 0, x: "-50%" }}
            exit={{ opacity: 0, y: 24, x: "-50%" }}
            transition={{ type: "spring", stiffness: 420, damping: 32 }}
          >
            <span className="text-[13px] font-medium">{t("telegram.unsaved")}</span>
            <Button variant="ghost" size="sm" onClick={reset}>
              {t("telegram.discard")}
            </Button>
            <Button variant="primary" size="sm" loading={patch.isPending} onClick={save}>
              {t("common.save")}
            </Button>
          </motion.div>
        ) : null}
      </AnimatePresence>
      <Confirm
        open={leave.status === "blocked"}
        onOpenChange={(open) => !open && leave.reset?.()}
        title={t("telegram.leaveTitle")}
        text={t("telegram.leaveText")}
        confirm={t("telegram.leaveConfirm")}
        danger
        onConfirm={() => leave.proceed?.()}
      />
    </>
  );
}

function InfrastructureCard({ v, draft, setDraft, dirty }: { v: View; draft: Schemas["AlertsConfig"]; setDraft: Dispatch<SetStateAction<Schemas["AlertsConfig"]>>; dirty: boolean }) {
  const patch = usePatchTelegram();
  const qc = useQueryClient();
  const toast = useToast();
  const { draft: adminID, setDraft: setAdminID } = useDraft(String(v.admin_id || ""));
  const [adminError, setAdminError] = useState("");
  const [, setChannelError] = useState("");
  const saveAdmin = (remove = false) => {
    const raw = adminID.trim();
    const id = remove ? 0 : Number(raw);
    if (!remove && (!/^\d+$/.test(raw) || !Number.isSafeInteger(id) || id <= 0)) {
      setAdminError(t("telegram.infra.adminIDError"));
      return;
    }
    patch.mutate({ admin_id: id }, {
      onSuccess: (r) => { setAdminID(String(r.admin_id || "")); setAdminError(""); toast.ok(t("settings.saved")); void qc.invalidateQueries({ queryKey: ["telegram-backup"] }); },
      onError: (e) => setAdminError(errorText(e)),
    });
  };
  const save = () => patch.mutate({ infrastructure: draft }, {
    onSuccess: (r) => { setDraft(r.infrastructure); setChannelError(""); toast.ok(t("telegram.infra.saved")); },
    onError: (e) => { setChannelError(errorText(e)); },
  });
  const change = (key: keyof typeof draft, value: boolean | string) => setDraft((d) => ({ ...d, [key]: value }));
  const event = (key: keyof typeof draft.events, value: boolean) => setDraft((d) => ({ ...d, events: { ...d.events, [key]: value } }));
  return (
    <section className="card glass">
      <div className="card-head"><div><h2 className="card-title">{t("telegram.infra.title")}</h2><div className="card-sub">{t("telegram.infra.subtitle")}</div></div></div>
      <div className="divide-y divide-[var(--hairline)]">
        <div className="flex flex-wrap items-center justify-between gap-3 py-4">
          <div><div className="text-sm font-medium">{t("telegram.infra.admin")}</div><div className="text-xs text-[var(--ink-500)]">{t("telegram.infra.adminHint")}</div></div>
          <Switch checked={draft.admin_enabled} label={t("telegram.infra.admin")} onChange={(on) => change("admin_enabled", on)} />
        </div>
        <div className="py-4">
          <Field label={t("telegram.infra.adminID")} htmlFor="tg-admin-id" hint={t("telegram.infra.adminIDHint")} error={adminError}>
            <input id="tg-admin-id" className="input mono" inputMode="numeric" value={adminID} onChange={(e) => setAdminID(e.target.value)} placeholder="123456789" maxLength={16} autoComplete="off" />
          </Field>
          <div className="flex flex-wrap items-center gap-3">
            <Button size="sm" loading={patch.isPending} disabled={adminID.trim() === String(v.admin_id || "")} onClick={() => saveAdmin()}>{t("telegram.infra.saveAdminID")}</Button>
            {v.admin_id > 0 ? <Button size="sm" variant="ghost" disabled={patch.isPending} onClick={() => saveAdmin(true)}>{t("telegram.infra.disconnect")}</Button> : null}
            {v.bot?.username ? <a className="text-xs underline" href={`https://t.me/${v.bot.username}`} target="_blank" rel="noopener noreferrer">{t("telegram.infra.openBot")}</a> : null}
          </div>
          <p className="mt-3 text-xs text-[var(--ink-500)]">{t("telegram.infra.adminManagementHint")}</p>
        </div>

      </div>
      <h3 className="mt-5 mb-2 text-sm font-semibold">{t("telegram.infra.events")}</h3>
      <p className="mb-3 text-xs text-[var(--ink-500)]">{t("telegram.infra.eventsHint")}</p>
      <div className="grid gap-2 sm:grid-cols-2">
        {([
          ["node", "eventNode"], ["warp", "eventWarp"], ["exit", "eventExit"], ["inbound", "eventInbound"],
          ["autotune", "eventAutotune"], ["autotune_recovery", "eventAutotuneRecovery"], ["tls", "eventTLS"], ["update", "eventUpdate"],
        ] as const).map(([key, label]) => <div key={key} className="panel-soft flex items-center justify-between gap-3 rounded-xl p-3"><span className="text-sm leading-5">{t(`telegram.infra.${label}`)}</span><Switch checked={draft.events[key]} label={t(`telegram.infra.${label}`)} onChange={(on) => event(key, on)} /></div>)}
      </div>
      <div className="mt-5 flex justify-end"><Button variant="primary" loading={patch.isPending} disabled={!dirty} onClick={save}>{t("common.save")}</Button></div>
    </section>
  );
}

// The database to the admin chat, encrypted with the admin's password (tgbackup).
function BackupCard() {
  const qc = useQueryClient();
  const toast = useToast();
  // While a backup is being made in the background, its end is looked for every few seconds.
  const q = useQuery({
    queryKey: ["telegram-backup"],
    queryFn: () => unwrap(api.GET("/api/v1/telegram/backup", {})),
    refetchInterval: (query) => (query.state.data?.sending ? 3000 : false),
  });
  const [password, setPassword] = useState("");
  const [shown, setShown] = useState(false);
  const [error, setError] = useState("");
  const done = (r: Schemas["BackupView"], text: string) => {
    qc.setQueryData(["telegram-backup"], r);
    setPassword("");
    setShown(false);
    setError("");
    toast.ok(text);
  };
  // A phrase nobody will guess: 32 characters from the browser's random source, shown so
  // the admin can keep it before it is saved.
  const generate = () => {
    const alphabet = "abcdefghijkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789";
    const bytes = crypto.getRandomValues(new Uint8Array(32));
    setPassword(Array.from(bytes, (x) => alphabet[x % alphabet.length]).join(""));
    setShown(true);
  };
  const fail = (e: unknown) => {
    if (e instanceof ApiError && Object.keys(e.fields).length) setError(Object.values(e.fields)[0] ?? "");
    else toast.error(errorText(e));
  };
  const patch = useMutation({
    mutationFn: (body: { enabled?: boolean; hour?: number; password?: string }) => unwrap(api.PATCH("/api/v1/telegram/backup", { body })),
    onSuccess: (r) => done(r, t("telegram.infra.backupSaved")),
    onError: fail,
  });
  const send = useMutation({
    mutationFn: () => unwrap(api.POST("/api/v1/telegram/backup/send", {})),
    onSuccess: (r) => done(r, t("telegram.infra.backupStarted")),
    onError: (e) => {
      fail(e);
      void qc.invalidateQueries({ queryKey: ["telegram-backup"] });
    },
  });
  const b = q.data;
  if (!b) return <Skeleton style={{ height: 220, borderRadius: 24 }} />;
  const size = b.last_size ? (b.last_size >= 1 << 20 ? `${(b.last_size / (1 << 20)).toFixed(1)} MB` : `${Math.max(1, Math.round(b.last_size / 1024))} KB`) : "";
  return (
    <section className="card glass">
      <div className="card-head">
        <div>
          <h2 className="card-title">{t("telegram.infra.backup")}</h2>
          <div className="card-sub">{t("telegram.infra.backupSub")}</div>
        </div>
      </div>
      {!b.admin_chat_set ? (
        <div className="banner warn mb-3">
          <span>{t("telegram.infra.backupNoChat")}</span>
        </div>
      ) : null}
      <div className="flex flex-wrap items-center justify-between gap-3 py-3">
        <div className="text-sm font-medium">{t("telegram.infra.backupOn")}</div>
        <Switch checked={b.enabled} label={t("telegram.infra.backupOn")} disabled={patch.isPending} onChange={(on) => patch.mutate(on && password ? { enabled: on, password } : { enabled: on })} />
      </div>
      <div className="grid gap-x-3 sm:grid-cols-[120px_1fr]">
        <Field label={t("telegram.infra.backupHour")} htmlFor="backup-hour">
          <Select id="backup-hour" className="input" value={b.hour} disabled={patch.isPending} onChange={(e) => patch.mutate({ hour: Number(e.target.value) })}>
            {Array.from({ length: 24 }, (_, h) => (
              <option key={h} value={h}>
                {String(h).padStart(2, "0")}:00
              </option>
            ))}
          </Select>
        </Field>
        <Field label={t("telegram.infra.backupPassword")} htmlFor="backup-password" hint={b.password_set ? t("telegram.infra.backupPasswordSet") : t("telegram.infra.backupPasswordHint")} error={error}>
          <div className="flex gap-2">
            <input id="backup-password" className="input mono" type={shown ? "text" : "password"} autoComplete="new-password" value={password} onChange={(e) => setPassword(e.target.value)} maxLength={256} aria-invalid={!!error} />
            <Button type="button" variant="ghost" onClick={generate}>
              {t("telegram.infra.backupGenerate")}
            </Button>
            <Button type="button" loading={patch.isPending} disabled={password.length === 0} onClick={() => patch.mutate({ password })}>
              {t("common.save")}
            </Button>
          </div>
        </Field>
      </div>
      {shown ? <div className="banner warn mb-3">{t("telegram.infra.backupKeepIt")}</div> : null}
      <div className="mt-2 flex flex-wrap items-center justify-between gap-3">
        <div className="text-xs text-[var(--ink-500)]">
          {b.last_ok ? t("telegram.infra.backupLast", { when: ago(b.last_ok), size }) : t("telegram.infra.backupNever")}
          {b.last_error ? <div className="text-[var(--berry-600)]">{t("telegram.infra.backupFailed", { error: tMaybe(`errors.api.${b.last_error}`) ?? b.last_error })}</div> : null}
        </div>
        <Button size="sm" loading={send.isPending || b.sending} disabled={!b.admin_chat_set || !b.password_set || b.sending} onClick={() => send.mutate()}>
          {b.sending ? t("telegram.infra.backupSending") : t("telegram.infra.backupSendNow")}
        </Button>
      </div>
      <div className="mt-4 text-xs text-[var(--ink-500)]">{t("telegram.infra.backupRestore")}</div>
      <pre className="code-block mt-1 text-xs">{"age -d -o mirai.tar.gz mirai-….tar.gz.age\nmirai restore mirai.tar.gz"}</pre>
    </section>
  );
}

function statusOf(v: View): { tone: "ok" | "warn" | "bad" | "off"; text: string } {
  if (!v.enabled) return { tone: "off", text: t("telegram.off") };
  if (v.running && !v.error) return { tone: "ok", text: t("telegram.running") };
  if (v.running) return { tone: "warn", text: t("telegram.reconnecting") };
  if (!v.error) return { tone: "off", text: t("telegram.starting") };
  return { tone: "bad", text: t("telegram.stopped") };
}

// Cards rise in turn, as on the other tabs.
const rise = (i: number) => ({ className: "card glass reveal", style: { "--i": i } as React.CSSProperties });
// Menu buttons slide to their new place when moved, shown or hidden.
const slide = { type: "spring", stiffness: 520, damping: 40 } as const;

function ConnectCard({ v }: { v: View }) {
  const patch = usePatchTelegram();
  const toast = useToast();
  const [token, setToken] = useState("");
  const [editing, setEditing] = useState(false);
  const [removing, setRemoving] = useState(false);
  const [error, setError] = useState("");
  const submit = (e: FormEvent) => {
    e.preventDefault();
    setError("");
    patch.mutate(
      { token: token.trim(), enabled: true },
      {
        onSuccess: (r) => {
          setToken("");
          setEditing(false);
          toast.ok(t("telegram.connected", { name: r.bot?.username ?? "" }));
        },
        onError: (err) => {
          if (err instanceof ApiError && Object.keys(err.fields).length) setError(Object.values(err.fields)[0] ?? "");
          else setError(errorText(err));
        },
      },
    );
  };
  const st = statusOf(v);
  const showForm = !v.token_set || editing;
  return (
    <section {...rise(0)}>
      <div className="card-head">
        <div>
          <h2 className="card-title">{t("telegram.connect")}</h2>
          <div className="card-sub">{t("telegram.connectSub")}</div>
        </div>
        {v.token_set ? (
          <Switch checked={v.enabled} label={t("telegram.enabled")} disabled={patch.isPending} onChange={(on) => patch.mutate({ enabled: on }, { onError: (e) => toast.error(errorText(e)) })} />
        ) : null}
      </div>
      {v.token_set && v.bot ? (
        <div className="panel-soft flex items-center gap-3 p-3">
          <span className="grid h-10 w-10 place-items-center rounded-xl bg-[var(--hover)] text-[var(--ink-700)]" aria-hidden>
            <Bot size={20} />
          </span>
          <div className="min-w-0 flex-1">
            <a className="font-semibold text-[var(--ink-900)] hover:underline" href={`https://t.me/${encodeURIComponent(v.bot.username)}`} target="_blank" rel="noreferrer noopener">
              @{v.bot.username}
            </a>
            <div className="truncate text-xs text-[var(--ink-500)]">{v.bot.name}</div>
          </div>
          <Pill tone={st.tone}>{st.text}</Pill>
        </div>
      ) : null}
      {v.error && v.enabled ? (
        <p className="mt-3 text-[13px] text-[var(--berry-600)]" role="alert">
          {tMaybe(`telegram.err.${v.error}`) ?? v.error}
        </p>
      ) : null}
      {v.token_set ? (
        <p className="mt-3 text-xs text-[var(--ink-500)]">
          {t("telegram.stats", { linked: v.linked, accounts: v.accounts })}
          {" · "}
          {v.mini_app_url ? t("telegram.miniAppOn") : t("telegram.miniAppNoCert")}
        </p>
      ) : null}
      {showForm ? (
        <form onSubmit={submit} className="mt-4" noValidate>
          {!v.token_set ? (
            <ol className="mb-4 flex list-decimal flex-col gap-1 pl-5 text-[13px] text-[var(--ink-600)]">
              <li>{t("telegram.step1")}</li>
              <li>{t("telegram.step2")}</li>
            </ol>
          ) : null}
          <Field label={t("telegram.token")} htmlFor="tg-token" hint={t("telegram.tokenHint")} error={error}>
            <input id="tg-token" className="input mono" type="password" autoComplete="off" spellCheck={false} value={token} onChange={(e) => setToken(e.target.value)} placeholder="123456789:AAH…" aria-invalid={!!error} />
          </Field>
          <div className="flex flex-wrap gap-2">
            <Button variant="primary" type="submit" loading={patch.isPending} disabled={!token.trim()}>
              <Link2 size={16} aria-hidden /> {t("telegram.connectButton")}
            </Button>
            {editing ? (
              <Button variant="ghost" onClick={() => setEditing(false)}>
                {t("common.cancel")}
              </Button>
            ) : null}
          </div>
        </form>
      ) : (
        <div className="mt-4 flex flex-wrap gap-2">
          <Button size="sm" onClick={() => setEditing(true)}>
            {t("telegram.changeToken")}
          </Button>
          <Button size="sm" variant="danger" onClick={() => setRemoving(true)}>
            <Trash2 size={16} aria-hidden /> {t("telegram.removeToken")}
          </Button>
        </div>
      )}
      <Confirm
        open={removing}
        onOpenChange={setRemoving}
        title={t("telegram.removeTitle")}
        text={t("telegram.removeText")}
        confirm={t("telegram.removeToken")}
        danger
        loading={patch.isPending}
        onConfirm={() => patch.mutate({ token: "" }, { onSuccess: () => setRemoving(false), onError: (e) => toast.error(errorText(e)) })}
      />
    </section>
  );
}

type RouteMode = Schemas["TelegramRoute"]["mode"];
const ROUTE_ICON = { direct: Globe, node: Network, proxy: Shield } as const;

// A node opens the tunnel to Telegram since 0.4.2; dev builds and nodes not heard from yet
// are given the benefit of the doubt (the check on saving tells).
function tunnels(version?: string): boolean {
  const m = /^(\d+)\.(\d+)\.(\d+)/.exec(version ?? "");
  if (!m) return true;
  const [a, b, c] = [Number(m[1]), Number(m[2]), Number(m[3])];
  return a > 0 || b > 4 || (b === 4 && c >= 2);
}

// How the bot reaches Telegram: straight, through a node of the panel or a proxy — for a
// server where Telegram is blocked.
function RouteCard({ v }: { v: View }) {
  const patch = usePatchTelegram();
  const toast = useToast();
  const nodes = useNodes();
  const saved = v.route;
  const {
    draft: { mode, nodeId },
    setDraft: setRoute,
  } = useDraft<{ mode: RouteMode; nodeId: number }>({ mode: saved.mode, nodeId: saved.node_id ?? 0 });
  const setMode = (m: RouteMode) => setRoute((d) => ({ ...d, mode: m }));
  const setNodeId = (n: number) => setRoute((d) => ({ ...d, nodeId: n }));
  const [proxy, setProxy] = useState("");
  const [error, setError] = useState("");
  const remote = (nodes.data ?? []).filter((n) => !n.local);
  const nodeName = (id?: number) => remote.find((n) => n.id === id)?.name ?? `#${id}`;
  const now =
    saved.mode === "node"
      ? t("telegram.routeNowNode", { name: nodeName(saved.node_id) })
      : saved.mode === "proxy"
        ? t("telegram.routeNowProxy", { proxy: saved.proxy ?? "" })
        : t("telegram.routeNowDirect");
  const changed = mode !== saved.mode || (mode === "node" && nodeId !== (saved.node_id ?? 0)) || (mode === "proxy" && proxy.trim() !== "");
  const ready = mode === "direct" || (mode === "node" && nodeId > 0) || (mode === "proxy" && (proxy.trim() !== "" || !!saved.proxy));
  const submit = (e: FormEvent) => {
    e.preventDefault();
    setError("");
    const route: Schemas["PatchTelegramInputBody"]["route"] =
      mode === "node" ? { mode, node_id: nodeId } : mode === "proxy" ? { mode, ...(proxy.trim() ? { proxy: proxy.trim() } : {}) } : { mode };
    patch.mutate(
      { route },
      {
        onSuccess: (r) => {
          setProxy("");
          const how = r.route.mode === "node" ? nodeName(r.route.node_id) : r.route.mode === "proxy" ? (r.route.proxy ?? "") : t("telegram.routeDirectShort");
          toast.ok(t("telegram.routeSaved", { how }));
        },
        onError: (err) => setError(err instanceof ApiError && Object.keys(err.fields).length ? (Object.values(err.fields)[0] ?? "") : errorText(err)),
      },
    );
  };
  const Icon = ROUTE_ICON[saved.mode];
  return (
    <section {...rise(1)}>
      <div className="card-head">
        <div>
          <h2 className="card-title">{t("telegram.routeTitle")}</h2>
          <div className="card-sub">{t("telegram.routeSub")}</div>
        </div>
      </div>
      <div className="panel-soft mb-4 flex items-center gap-3 p-3">
        <span className="grid h-10 w-10 shrink-0 place-items-center rounded-xl bg-[var(--hover)] text-[var(--ink-700)]" aria-hidden>
          <Icon size={20} />
        </span>
        <div className="min-w-0">
          <div className="text-xs text-[var(--ink-500)]">{t("telegram.routeNow")}</div>
          <div className="truncate text-[13px] font-semibold">{now}</div>
        </div>
      </div>
      <form onSubmit={submit} noValidate>
        <div className="mb-4">
          <Segmented
            value={mode}
            onChange={(m) => {
              setMode(m);
              setError("");
            }}
            label={t("telegram.routeTitle")}
            options={[
              { value: "direct", label: t("telegram.routeDirect") },
              { value: "node", label: t("telegram.routeNode") },
              { value: "proxy", label: t("telegram.routeProxy") },
            ]}
          />
        </div>
        {mode === "direct" ? <p className="mb-4 text-xs text-[var(--ink-500)]">{t("telegram.routeDirectHint")}</p> : null}
        {mode === "node" ? (
          nodes.isPending ? (
            <Skeleton style={{ height: 40, borderRadius: 12, maxWidth: 320 }} />
          ) : remote.length === 0 ? (
            <div className="banner warn mb-4 flex-wrap" role="status">
              <span className="min-w-0 flex-1">{t("telegram.routeNoNodes")}</span>
              <Link to="/nodes" className="btn btn-glass btn-sm">
                {t("telegram.routeOpenNodes")}
              </Link>
            </div>
          ) : (
            <Field label={t("telegram.routeNodeLabel")} htmlFor="tg-route-node" hint={t("telegram.routeNodeHint")} error={error}>
              <Select id="tg-route-node" className="input max-w-[320px]" value={nodeId} onChange={(e) => setNodeId(Number(e.target.value))} aria-invalid={!!error}>
                <option value={0} disabled>
                  {t("telegram.routeNodePick")}
                </option>
                {remote.map((n) => (
                  <option key={n.id} value={n.id} disabled={!tunnels(n.version)}>
                    {tunnels(n.version) ? n.name : t("telegram.routeNodeOld", { name: n.name })}
                  </option>
                ))}
              </Select>
            </Field>
          )
        ) : null}
        {mode === "proxy" ? (
          <Field label={t("telegram.routeProxyLabel")} htmlFor="tg-route-proxy" hint={saved.proxy ? t("telegram.routeProxyKeep", { proxy: saved.proxy }) : t("telegram.routeProxyHint")} error={error}>
            <input
              id="tg-route-proxy"
              className="input mono"
              type="text"
              autoComplete="off"
              spellCheck={false}
              value={proxy}
              onChange={(e) => setProxy(e.target.value)}
              placeholder="socks5://user:pass@203.0.113.5:1080"
              maxLength={512}
              aria-invalid={!!error}
            />
          </Field>
        ) : null}
        {error && mode === "direct" ? (
          <p className="mb-3 text-xs text-[var(--berry-600)]" role="alert">
            {error}
          </p>
        ) : null}
        {mode === "node" && !nodes.isPending && remote.length === 0 ? null : (
          <Button variant="primary" type="submit" loading={patch.isPending} disabled={!changed || !ready}>
            {mode === "direct" ? t("common.save") : t("telegram.routeSave")}
          </Button>
        )}
      </form>
    </section>
  );
}



function MenuActionIcon({ action }: { action: string }) {
 const Icon = ({ profile: UserRound, buy: Wallet, devices: Smartphone, connect: ArrowUpRight, renew: Wallet, support: LifeBuoy, app: Globe, url: Link2, page: LayoutList } as Record<string, typeof UserRound>)[action] ?? LayoutList;
 return <Icon size={17} strokeWidth={1.6} className="shrink-0 text-[var(--ink-500)]" aria-hidden />;
}

function actionLabel(a: string): string {
  return a === "buy" ? "Купить подписку" : tMaybe(`telegram.action.${a}`) ?? a;
}

function MenuReorderItem({ id, reduce, children }: { id: string; reduce: boolean; children: ReactNode }) {
  const controls = useDragControls();
  return <Reorder.Item value={id} dragListener={false} dragMomentum={false} dragControls={controls}
    onPointerDown={(event) => { if (event.button === 0 && (event.target as HTMLElement).closest("[data-menu-drag]")) { event.preventDefault(); controls.start(event); } }}
    className="panel-soft p-3 relative" style={{ position: "relative" }}
    whileDrag={{ zIndex: 10, scale: reduce ? 1 : 1.015, boxShadow: "0 12px 32px rgba(0,0,0,.18)", cursor: "grabbing" }}
    initial={reduce ? false : { opacity: 0, scale: .96 }} animate={{ opacity: 1, scale: 1 }}
    exit={{ opacity: 0 }} transition={reduce ? { duration: 0 } : slide}>
    {children}
  </Reorder.Item>;
}

function MenuCard({ draft, setDraft }: { draft: Config; setDraft: (c: Config) => void }) {
  const set = (i: number, patch: Partial<MenuButton>) => setDraft({ ...draft, buttons: draft.buttons.map((b, j) => (j === i ? { ...b, ...patch } : b)) });
  const move = (i: number, d: -1 | 1) => {
    const list = [...draft.buttons];
    [list[i], list[i + d]] = [list[i + d]!, list[i]!];
    setDraft({ ...draft, buttons: list });
  };
  const add = (action: "url" | "page") => {
    setDraft({
      ...draft,
      buttons: [...draft.buttons, { id: `c${Date.now().toString(36)}`, action, label: action === "url" ? t("telegram.newLink") : t("telegram.newPage"), on: true, row: false, url: action === "url" ? "https://" : undefined, text: action === "page" ? "" : undefined }],
    });
  };

  const reduce = useReducedMotion();
  return (
    <section {...rise(1)}>
      <div className="card-head">
        <div>
          <h2 className="card-title">{t("telegram.menu")}</h2>
          <div className="card-sub">{t("telegram.menuSub")}</div>
        </div>
      </div>
      <Reorder.Group axis="y" values={draft.buttons.map((b) => b.id)} onReorder={(ids) => setDraft({ ...draft, buttons: ids.map((id) => draft.buttons.find((b) => b.id === id)!) })} className="flex flex-col gap-2">
        <AnimatePresence initial={false}>
          {draft.buttons.map((b, i) => (
            <MenuReorderItem key={b.id} id={b.id} reduce={!!reduce}>
              <div className="flex items-center gap-2">
                <MenuActionIcon action={b.action} />
                <Switch checked={b.on} label={t("telegram.showButton", { name: b.label })} onChange={(on) => set(i, { on })} />
                <input className="input h-10 min-w-0 flex-1" value={b.label} maxLength={40} onChange={(e) => set(i, { label: e.target.value })} aria-label={t("telegram.buttonLabel")} />
                <button type="button" className="icon-btn menu-drag-handle" data-menu-drag style={{ touchAction: "none" }}
                  aria-label={t("telegram.dragButton", { name: b.label })} title={t("telegram.dragHint")}
                  onKeyDown={(e) => { if (e.key === "ArrowUp" && i > 0) { e.preventDefault(); move(i, -1); } else if (e.key === "ArrowDown" && i < draft.buttons.length - 1) { e.preventDefault(); move(i, 1); } }}>
                  <GripVertical size={18} aria-hidden />
                </button>
                {(
                  <button type="button" className="icon-btn" onClick={() => setDraft({ ...draft, buttons: draft.buttons.filter((_, j) => j !== i) })} aria-label={t("telegram.deleteButton", { name: b.label })}>
                    <Trash2 size={16} />
                  </button>
                )}
              </div>
              <div className="mt-2 flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-[var(--ink-500)]">
                <span>{actionLabel(b.action)}</span>
                {i > 0 ? (
                  <label className="flex items-center gap-1.5">
                    <input type="checkbox" className="check" checked={b.row} onChange={(e) => set(i, { row: e.target.checked })} /> {t("telegram.sameRow")}
                  </label>
                ) : null}
              </div>
              {b.action === "url" ? <input className="input mono mt-2" value={b.url ?? ""} onChange={(e) => set(i, { url: e.target.value })} placeholder="https://… / tg://…" aria-label={t("telegram.buttonUrl")} /> : null}
              {b.action === "page" ? (
                <textarea className="input mt-2" value={b.text ?? ""} maxLength={3000} onChange={(e) => set(i, { text: e.target.value })} placeholder={t("telegram.pagePlaceholder")} aria-label={t("telegram.pageText")} />
              ) : null}
            </MenuReorderItem>
          ))}
        </AnimatePresence>
      </Reorder.Group>
      <div className="mt-3 flex flex-wrap gap-2">
        {(["profile", "renew", "buy"] as const).filter(action => !draft.buttons.some(b => b.action === action)).map(action => <button key={action} type="button" className="chip-btn" disabled={draft.buttons.length >= 20} onClick={() => setDraft({...draft, buttons: [...draft.buttons, {id: action, action, label: ({profile: "👤 Профиль", buy: "🛒 Купить", renew: "💳 Продлить"})[action], on: true, row: false}]})}>+ {({profile: "Профиль", buy: "Купить", renew: "Продлить"})[action]}</button>)}
        <button type="button" className="chip-btn" onClick={() => add("url")} disabled={draft.buttons.length >= 20}>
          <Plus size={14} className="mr-1 inline" aria-hidden />
          {t("telegram.addLink")}
        </button>
        <button type="button" className="chip-btn" onClick={() => add("page")} disabled={draft.buttons.length >= 20}>
          <Plus size={14} className="mr-1 inline" aria-hidden />
          {t("telegram.addPage")}
        </button>
      </div>
    </section>
  );
}

const TEXTS: { key: TextKey; label: string }[] = [
  { key: "welcome", label: "telegram.text.welcome" },
  { key: "renew", label: "telegram.text.renew" },
  { key: "expiring", label: "telegram.text.expiring" },
  { key: "expired", label: "telegram.text.expired" },
  { key: "traffic_90", label: "telegram.text.traffic_90" },
  { key: "traffic_end", label: "telegram.text.traffic_end" },
];

function TextsCard({ draft, setDraft, defaults }: { draft: Config; setDraft: (c: Config) => void; defaults: Schemas["Texts"] }) {
  return (
    <section {...rise(2)}>
      <div className="card-head">
        <div>
          <h2 className="card-title">{t("telegram.texts")}</h2>
          <div className="card-sub">{t("telegram.textsSub")}</div>
        </div>
      </div>
      <Field label={t("telegram.lang")} hint={t("telegram.langHint")}>
        <Segmented
          label={t("telegram.lang")}
          value={draft.lang}
          onChange={(lang) => setDraft({ ...draft, lang })}
          options={[
            { value: "ru", label: "Русский" },
            { value: "en", label: "English" },
          ]}
        />
      </Field>
      {TEXTS.map(({ key, label }) => (
        <Field key={key} label={key === "welcome" ? "Приветствие /start · главное меню" : tMaybe(label) ?? key} hint={key === "welcome" ? "Информация о вашем VPN. Под сообщением отображаются кнопки главного меню; статистика находится в профиле и подписках." : undefined} htmlFor={`tg-${key}`}>
          <textarea id={`tg-${key}`} className="input" rows={key === "main" || key === "welcome" ? 11 : 7} maxLength={3000} value={draft.texts[key]} placeholder={defaults[key]} onChange={(e) => setDraft({ ...draft, texts: { ...draft.texts, [key]: e.target.value } })} />
          <p className="mt-2 text-xs text-[var(--ink-500)]">Можно выделять заголовки: &lt;b&gt;жирный&lt;/b&gt;, &lt;i&gt;курсив&lt;/i&gt;, &lt;code&gt;текст для копирования&lt;/code&gt;.</p>
        </Field>
      ))}
      <p className="text-xs text-[var(--ink-500)]">{t("telegram.variables")} · {"{subscription_url}"} — ссылка подписки</p>
    </section>
  );
}

const NOTICES: (keyof Schemas["Notify"])[] = ["expire_3d", "expire_1d", "expired", "traffic_90", "traffic_100"];

function OptionsCard({ draft, setDraft, v }: { draft: Config; setDraft: (c: Config) => void; v: View }) {
  const row = (title: string, sub: string, on: boolean, change: (v: boolean) => void) => (
    <li key={title} className="flex items-start justify-between gap-4 py-3">
      <div className="min-w-0">
        <div className="text-[13px] font-medium">{title}</div>
        <div className="mt-1 text-xs text-[var(--ink-500)]">{sub}</div>
      </div>
      <Switch checked={on} label={title} onChange={change} />
    </li>
  );
  return (
    <section {...rise(3)}>
      <div className="card-head">
        <div>
          <h2 className="card-title">{t("telegram.options")}</h2>
          <div className="card-sub">{t("telegram.optionsSub")}</div>
        </div>
      </div>
      <ul className="row-list">
        {row(t("telegram.miniApp"), v.mini_app_url ? t("telegram.miniAppSub") : t("telegram.miniAppNoCert"), draft.mini_app, (on) => setDraft({ ...draft, mini_app: on }))}
        {row(t("telegram.cleanChat"), t("telegram.cleanChatSub"), draft.clean_chat, (on) => setDraft({ ...draft, clean_chat: on }))}
        {row(t("telegram.quietNight"), t("telegram.quietNightSub"), draft.quiet_night, (on) => setDraft({ ...draft, quiet_night: on }))}
        {NOTICES.map((k) => row(t(`telegram.notice.${k}`), t("telegram.noticeSub"), draft.notify[k], (on) => setDraft({ ...draft, notify: { ...draft.notify, [k]: on } })))}
      </ul>
    </section>
  );
}

function BroadcastCard({ v }: { v: View }) {
  const toast = useToast();
  const qc = useQueryClient();
  const [text, setText] = useState("");
  const [confirm, setConfirm] = useState(false);
  const send = useMutation({
    mutationFn: () => unwrap(api.POST("/api/v1/telegram/broadcast", { body: { text } })),
    onSuccess: (r) => {
      toast.ok(t("telegram.broadcastSent", { n: r.queued }));
      setText("");
      setConfirm(false);
      void qc.invalidateQueries({ queryKey: qk.telegram });
    },
    onError: (e) => toast.error(errorText(e)),
  });
  const busy = !!v.broadcast?.active;
  const ready = v.running && v.accounts > 0 && !busy;
  return (
    <section {...rise(4)}>
      <div className="card-head">
        <div>
          <h2 className="card-title">{t("telegram.broadcast")}</h2>
          <div className="card-sub">{t("telegram.broadcastSub", { n: v.accounts })}</div>
        </div>
      </div>
      <textarea className="input" rows={4} maxLength={3500} value={text} onChange={(e) => setText(e.target.value)} placeholder={t("telegram.broadcastPlaceholder")} aria-label={t("telegram.broadcast")} disabled={!v.running} />
      <div className="mt-3 flex flex-wrap items-center gap-3">
        <Button variant="primary" disabled={!ready || !text.trim()} onClick={() => setConfirm(true)}>
          <Send size={16} aria-hidden /> {t("telegram.broadcastButton")}
        </Button>
        <span className="text-xs text-[var(--ink-500)]">{busy ? t("telegram.broadcasting") : !v.running ? t("telegram.broadcastOff") : t("telegram.broadcastHint")}</span>
      </div>
      {v.broadcast && <BroadcastProgress b={v.broadcast} />}
      <Confirm
        open={confirm}
        onOpenChange={setConfirm}
        title={t("telegram.broadcastTitle", { n: v.accounts })}
        text={t("telegram.broadcastText")}
        confirm={t("telegram.broadcastButton")}
        loading={send.isPending}
        onConfirm={() => send.mutate()}
      />
    </section>
  );
}

/** How far the last broadcast went; the bot sends up to 20 a second. */
function BroadcastProgress({ b }: { b: Schemas["TelegramBroadcast"] }) {
  const done = b.sent + b.failed;
  const sec = Math.ceil((b.total - done) / 20);
  const eta = sec < 60 ? t("telegram.broadcastEtaSec", { n: Math.max(1, sec) }) : t("telegram.broadcastEtaMin", { n: Math.ceil(sec / 60) });
  return (
    <div className="mt-4 rounded-2xl border border-[var(--hairline)] bg-[var(--glass-strong)] p-3" aria-live="polite">
      <div className="flex items-baseline justify-between gap-3 text-[13px]">
        <span className="font-medium">{b.active ? t("telegram.broadcastGoing") : t("telegram.broadcastLast", { when: ago(new Date(b.started * 1000).toISOString()) })}</span>
        <span className="tabular-nums text-[var(--ink-500)]">{t("telegram.broadcastCount", { done: num(b.sent), total: num(b.total) })}</span>
      </div>
      <div className="mt-2" role="progressbar" aria-label={t("telegram.broadcast")} aria-valuemin={0} aria-valuemax={b.total} aria-valuenow={done}>
        <Bar pct={b.total ? (done / b.total) * 100 : 100} />
      </div>
      {(b.active || b.failed > 0) && (
        <div className="mt-2 flex flex-wrap gap-x-3 gap-y-1 text-xs text-[var(--ink-500)]">
          {b.active && <span>{eta}</span>}
          {b.failed > 0 && <span>{t("telegram.broadcastFailed", { n: num(b.failed) })}</span>}
        </div>
      )}
    </div>
  );
}

/** The main menu as a subscriber sees it in Telegram, with sample data. */
function Preview({ draft, v }: { draft: Config; v: View }) {
  const [screen, setScreen] = useState<"main" | "profile" | "subscriptions" | "subscription" | "devices">("main");
  const settings = useSettings();
  const brand = settings.data?.brand || "VPN";
  const support = !!settings.data?.support_url;
  const locale = useLocale();
  const sample: Record<string, string> = useMemo(
    () => ({
      brand,
      name: t("telegram.sample.name"),
      state: t("telegram.sample.state"),
      term: t("telegram.sample.term"),
      traffic: t("telegram.sample.traffic"),
      devices: t("telegram.sample.devices"),
      until: t("telegram.sample.until"),
      days: t("telegram.sample.days"),
      used: t("telegram.sample.used"),
      left: t("telegram.sample.left"),
      limit: t("telegram.sample.limit"),
      reset: t("telegram.sample.reset"),
    }),
    // The sample texts are translated: they change with the language.
    [brand, locale],
  );
  const mainText = (draft.texts.welcome || v.defaults.welcome).replace(/\{(\w+)\}/g, (m, k: string) => sample[k] ?? m);
  const text = screen === "subscriptions" ? "📋 Мои подписки\nВыберите подписку" : screen === "devices" ? "📱 Устройства · 2 из 3\n\n1. iPhone · Happ\n2. Windows · INCY" : screen === "profile" ? t("telegram.sample.profile", { name: sample.name ?? "" }) : screen === "subscription" ? t("telegram.sample.subscriptions", { name: sample.name ?? "", term: sample.term ?? "", traffic: sample.traffic ?? "" }) + "\n\n🔗 Ссылка для подключения\nhttps://subs.example.com/A7b2X9mQ4\n\nСкопируйте ссылку и добавьте её в Happ, INCY или другое VPN-приложение." : mainText;
  const reduce = useReducedMotion();
  const rows: MenuButton[][] = [];
  const previewButtons: MenuButton[] = screen === "devices" ? [{id:"device-one",action:"page",label:"📱 iPhone · Happ",on:true,row:false},{id:"device-two",action:"page",label:"📱 Windows · INCY",on:true,row:false},{id:"clear-all",action:"page",label:"🧹 Очистить все",on:true,row:false},{id: "subscription", action: "page", label: "← Подписка", on: true, row: false}] : screen === "profile" ? [{id: "subscriptions", action: "page", label: "📋 Мои подписки", on: true, row: false}, {id: "home", action: "page", label: "← Главное меню", on: true, row: false}] : screen === "subscriptions" ? [{id: "subscription", action: "page", label: "📋 Подписка · Анна", on: true, row: false}, {id: "profile", action: "profile", label: "← Профиль", on: true, row: false}] : screen === "subscription" ? [{id: "devices", action: "devices", label: "📱 Устройства (2)", on: true, row: false}, {id: "subscriptions", action: "page", label: "← Мои подписки", on: true, row: false}] : draft.buttons;
  for (const b of previewButtons) {
    if (!b.on || (b.action === "support" && !support) || (b.action === "app" && !(draft.mini_app && v.mini_app_url))) continue;
    const last = rows[rows.length - 1];
    if (b.row && last && last.length < 3) last.push(b);
    else rows.push([b]);
  }
  if (screen === "main" && draft.admin.enabled) rows.push([{id: "admin-preview", action: "page", label: "⚙️ Админ-панель · только ваш ID", on: true, row: false}]);
  return (
    <section {...rise(1)} aria-label={t("telegram.preview")}>
      <div className="card-head">
        <div>
          <h2 className="card-title">{t("telegram.preview")}</h2>
          <div className="card-sub">{t("telegram.previewSub")}</div>
        </div>
      </div>
      <div className="flex flex-wrap gap-2 mb-3">
        {(["main", "profile", "subscriptions"] as const).map((item) => <button type="button" key={item} className="chip-btn" aria-pressed={screen === item} onClick={() => setScreen(item)}>{item === "subscriptions" ? "Мои подписки" : t(`telegram.previewScreens.${item}`)}</button>)}
      </div>
      <div className="tg-chat">
        <div className="tg-bubble"><TelegramText text={text} /></div>
        <motion.div className="tg-keyboard" layout={!reduce} transition={slide}>
          <AnimatePresence initial={false} mode="popLayout">
            {rows.map((r) => (
              <motion.div key={r[0]!.id} className="tg-row" layout={!reduce} transition={slide}>
                <AnimatePresence initial={false} mode="popLayout">
                  {r.map((b) => (
                    <motion.button type="button" onClick={() => { if (b.action === "profile") setScreen("profile"); else if (b.id === "subscription") setScreen("subscription"); else if (b.id === "subscriptions") setScreen("subscriptions"); else if (b.id === "home") setScreen("main"); else if (b.action === "devices") setScreen("devices"); }}
                      key={b.id}
                      className="tg-btn"
                      layout={!reduce}
                      initial={reduce ? false : { opacity: 0, scale: 0.85 }}
                      animate={{ opacity: 1, scale: 1 }}
                      exit={reduce ? { opacity: 0 } : { opacity: 0, scale: 0.85 }}
                      transition={slide}
                    >
                      {b.label}
                    </motion.button>
                  ))}
                </AnimatePresence>
              </motion.div>
            ))}
          </AnimatePresence>
        </motion.div>
      </div>
      {!v.mini_app_url && draft.buttons.some((b) => b.action === "app" && b.on) ? (
        <p className="mt-3 flex items-start gap-2 text-xs text-[var(--ink-500)]">
          <TriangleAlert size={14} className="mt-0.5 shrink-0 text-[var(--honey-600)]" aria-hidden /> {t("telegram.miniAppNoCert")}
        </p>
      ) : null}
    </section>
  );
}

function AdminMenuCard({draft,setDraft}:{draft:Config;setDraft:(c:Config)=>void}) {
 const reduce=useReducedMotion();const buttons=(draft.admin.buttons ?? []).filter(b=>b.action!=="grant");
 const update=(next:typeof buttons)=>setDraft({...draft,admin:{...draft.admin,buttons:next,version:2,users:next.some(b=>b.action==="users"&&b.on),subscriptions:next.some(b=>b.action==="subscriptions"&&b.on),search:next.some(b=>b.action==="search"&&b.on)}});
 const change=(id:string,values:Partial<(typeof buttons)[number]>)=>update(buttons.map(b=>b.id===id?{...b,...values}:b));
 const labels={users:"👥 Пользователи",subscriptions:"📋 Все подписки",search:"🔎 Поиск",grant:"🎁 Выдать подписку"};
 const rows:typeof buttons[]=[];for(const b of buttons){if(!b.on)continue;const last=rows[rows.length-1];if(b.row&&last&&last.length<3)last.push(b);else rows.push([b]);}
 return <div className="grid items-start gap-4 xl:grid-cols-[minmax(0,1fr)_360px]"><section className="card glass"><div className="flex items-center gap-3 mb-4"><h2 className="card-title flex-1">Админ-меню</h2><Switch checked={draft.admin.enabled} label="Админ-меню" onChange={enabled=>setDraft({...draft,admin:{...draft.admin,enabled}})}/></div><p className="card-sub mb-4">Перетаскивайте кнопки за маркер. Изменения сохраняются кнопкой «Сохранить».</p>
 <Reorder.Group axis="y" values={buttons.map(b=>b.id)} onReorder={ids=>update(ids.map(id=>buttons.find(b=>b.id===id)!))} className="flex flex-col gap-2"><AnimatePresence initial={false}>{buttons.map((b,i)=><MenuReorderItem key={b.id} id={b.id} reduce={!!reduce}><div className="flex items-center gap-2"><Switch checked={b.on} label={b.label} onChange={on=>change(b.id,{on})}/><input className="input min-w-0 flex-1" maxLength={40} value={b.label} onChange={e=>change(b.id,{label:e.target.value})} aria-label="Название кнопки"/><button type="button" className="icon-btn menu-drag-handle" data-menu-drag style={{touchAction:"none"}} aria-label="Перетащить кнопку"><GripVertical size={18}/></button><button type="button" className="icon-btn" aria-label="Удалить кнопку" onClick={()=>update(buttons.filter(item=>item.id!==b.id))}><Trash2 size={16}/></button></div>{i>0?<label className="flex items-center gap-2 mt-2 text-xs"><input type="checkbox" className="check" checked={b.row} onChange={e=>change(b.id,{row:e.target.checked})}/>В одном ряду с предыдущей</label>:null}</MenuReorderItem>)}</AnimatePresence></Reorder.Group>
 <div className="flex flex-wrap gap-2 mt-3">{(["users","subscriptions","search"] as const).filter(action=>!buttons.some(b=>b.action===action)).map(action=><button type="button" className="chip-btn" key={action} onClick={()=>update([...buttons,{id:action,action,label:labels[action],on:true,row:false}])}>+ {labels[action]}</button>)}</div><div className="flex items-center gap-3 mt-5"><span>Выдавать подписку в профиле пользователя</span><Switch checked={draft.admin.grant} label="Выдача подписки" onChange={grant=>setDraft({...draft,admin:{...draft.admin,grant}})}/></div><div className="flex items-center gap-3 mt-5"><span>Статистика в шапке</span><Switch checked={draft.admin.statistics} label="Статистика" onChange={statistics=>setDraft({...draft,admin:{...draft.admin,statistics}})}/></div></section>
 <section className="card glass"><h2 className="card-title mb-3">Превью · только ваш Telegram ID</h2><div className="tg-chat"><div className="tg-bubble">{"⚙️ Админ-панель Mirai\n\nУправление пользователями и подписками"}{draft.admin.statistics?"\nАктивные: 24 · Истекают: 3\nИстекли: 2 · Лимит: 1 · Заморожены: 0":""}</div><div className="tg-keyboard">{draft.admin.enabled?rows.map(row=><div className="tg-row" key={row[0]!.id}>{row.map(b=><button type="button" className="tg-btn" key={b.id}>{b.label}</button>)}</div>):<div className="tg-bubble">Админ-меню отключено</div>}<div className="tg-row"><button type="button" className="tg-btn">← Главное меню</button></div></div></div></section></div>;
}

function DeviceResetCard({v}: {v: View}) {
 const {draft, setDraft, dirty} = useDraft(v.device_reset);
 const patch = usePatchTelegram(); const toast = useToast();
 return <section className="card glass mt-4"><h2 className="card-title">Очистка устройств</h2><p className="card-sub">Общий лимит для каждой подписки. Для администратора — без ограничений. Очистка всех устройств считается одной операцией.</p><div className="grid gap-4 mt-4 md:grid-cols-2"><div className="flex items-center gap-3"><span>По одному устройству</span><Switch checked={draft.single} label="Очистка по одному" onChange={single => setDraft({...draft, single})}/></div><div className="flex items-center gap-3"><span>Все устройства сразу</span><Switch checked={draft.all} label="Очистка всех устройств" onChange={all => setDraft({...draft, all})}/></div><Field label="Количество очисток"><input className="input" type="number" min={1} max={1000} value={draft.limit} onChange={e => setDraft({...draft, limit: Number(e.target.value)})}/></Field><Field label="Период в днях"><input className="input" type="number" min={1} max={365} value={draft.period_days} onChange={e => setDraft({...draft, period_days: Number(e.target.value)})}/></Field></div><p className="card-sub mt-3">Например: 4 очистки за 30 дней. Каждая использованная очистка снова доступна через этот период.</p><Button className="mt-3" disabled={!dirty || patch.isPending || draft.limit < 1 || draft.limit > 1000 || draft.period_days < 1 || draft.period_days > 365} onClick={() => patch.mutate({device_reset: draft}, {onSuccess: () => toast.ok(t("telegram.saved")), onError: e => toast.error(errorText(e))})}>Сохранить лимиты</Button></section>;
}

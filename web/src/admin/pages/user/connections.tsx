import { useMemo, useState } from "react";
import { errorText, type Schemas, type User } from "../../../api/client";
import { useUserConnections } from "../../../api/hooks";
import { Drawer } from "../../../components/overlay";
import { ErrorState, Skeleton } from "../../../components/ui";
import { t } from "../../../i18n";
import { bytes, dateShort, time } from "../../../lib/format";

type Conn = Schemas["ConnectionView"];

/** The user's connection log: where the traffic goes, from which address, through which inbound. */
export function ConnectionsDialog({ u, open, onOpenChange }: { u: User; open: boolean; onOpenChange: (v: boolean) => void }) {
  const log = useUserConnections(u.id, open);
  const [filter, setFilter] = useState("");
  const all = log.data?.conns ?? [];
  const list = useMemo(() => {
    const q = filter.trim().toLowerCase();
    return q ? all.filter((c) => [c.dest, c.ip, c.inbound, c.node, c.network].some((v) => v.toLowerCase().includes(q))) : all;
  }, [all, filter]);
  const live = all.filter((c) => !c.ended_at).length;
  const down = log.data?.unavailable ?? [];

  return (
    <Drawer presentation="modal" modalWidth="protocol" open={open} onOpenChange={onOpenChange} title={t("userDrawer.connectionsTitle", { name: u.name })} meta={log.data ? <span>{t("userDrawer.connectionsCount", { open: live, total: all.length })}</span> : undefined}>
      <div className="flex flex-col gap-3 py-4">
        <input className="input" value={filter} onChange={(e) => setFilter(e.target.value)} placeholder={t("userDrawer.connectionsFilter")} aria-label={t("userDrawer.connectionsFilter")} spellCheck={false} />
        {down.length > 0 ? <p className="text-xs text-[var(--bad)]">{t("userDrawer.connectionsUnavailable", { nodes: down.join(", ") })}</p> : null}
        {log.isPending ? (
          <Skeleton style={{ height: 160, borderRadius: 16 }} />
        ) : log.data === undefined ? (
          <ErrorState text={errorText(log.error)} onRetry={() => void log.refetch()} />
        ) : list.length === 0 ? (
          <p className="text-[13px] text-[var(--ink-500)]">{all.length === 0 ? t("userDrawer.connectionsEmpty") : t("userDrawer.connectionsNoMatch")}</p>
        ) : (
          <ul className="flex flex-col gap-1.5">
            {list.map((c, i) => (
              <Row key={`${c.node_id}/${c.network}/${c.ip}/${c.dest}/${c.started_at}/${i}`} c={c} />
            ))}
          </ul>
        )}
        <p className="text-xs text-[var(--ink-500)]">{t("userDrawer.connectionsNote")}</p>
      </div>
    </Drawer>
  );
}

function Row({ c }: { c: Conn }) {
  return (
    <li className="panel-soft grid grid-cols-[minmax(0,1fr)_auto] items-center gap-x-3 gap-y-0.5 px-3 py-2 text-xs">
      <div className="mono truncate text-[13px] font-medium text-[var(--ink-900)]" title={c.dest}>
        {c.dest}
      </div>
      <div className="num text-right text-[var(--ink-600)]">
        ↑ {bytes(c.up)} · ↓ {bytes(c.down)}
      </div>
      <div className="truncate text-[var(--ink-500)]">
        <span className="uppercase">{c.network}</span> · <span className="mono">{c.ip}</span> · {c.inbound} · {c.node}
      </div>
      <div className="text-right text-[var(--ink-500)]">
        {dateShort(c.started_at)} {time(c.started_at)}
        {" — "}
        {c.ended_at ? time(c.ended_at) : <span className="text-[var(--leaf-700)]">{t("userDrawer.connectionsOpen")}</span>}
      </div>
    </li>
  );
}

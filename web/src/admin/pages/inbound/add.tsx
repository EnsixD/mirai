import { useMutation, useQueryClient } from "@tanstack/react-query";
import { CircleCheck, Network } from "lucide-react";
import { useEffect, useState, type FormEvent } from "react";
import { api, ApiError, errorText, unwrap, type Schemas } from "../../../api/client";
import { qk, usePresets } from "../../../api/hooks";
import { Drawer } from "../../../components/overlay";
import { QueryBoundary } from "../../../components/query";
import { useToast } from "../../../components/toast";
import { Button, Field, Skeleton } from "../../../components/ui";
import { t } from "../../../i18n";
import { Editor, LimitBadges, LimitNotes, ValidateResult, preloadEditor, presetSummary, presetTitle, useValidate } from "./shared";

export function AddModal({ open, onOpenChange, nodeId, nodeName }: { open: boolean; onOpenChange: (v: boolean) => void; nodeId: number; nodeName?: string }) {
  const presets = usePresets();
  const qc = useQueryClient();
  const toast = useToast();
  const validate = useValidate();
  const [preset, setPreset] = useState<string>("vless_reality_tcp");
  const [port, setPort] = useState("");
  const [config, setConfig] = useState("");
  const [errors, setErrors] = useState<Record<string, string>>({});
  useEffect(() => {
    if (open) {
      setPort("");
      setErrors({});
      setConfig(t("inbounds.customSkeleton"));
      validate.reset();
    }
    // `validate` changes identity on every render; reset only when the modal opens.
  }, [open]);
  const editConfig = (v: string) => {
    setConfig(v);
    setErrors(({ config: _, ...rest }) => rest);
    validate.reset();
  };
  const create = useMutation({
    mutationFn: (body: Schemas["CreateInboundInputBody"]) => unwrap(api.POST("/api/v1/inbounds", { body })),
    onSuccess: (r) => {
      void qc.invalidateQueries({ queryKey: qk.inbounds });
      toast.ok(t("inbounds.addedOn", { name: r.sub_name, port: r.port }));
      onOpenChange(false);
    },
    onError: (e) => {
      validate.reset();
      if (e instanceof ApiError && Object.keys(e.fields).length) setErrors(e.fields);
      else toast.error(errorText(e));
    },
  });
  const custom = preset === "custom";
  useEffect(() => {
    if (custom && open) preloadEditor();
  }, [custom, open]);
  const chosen = presets.data?.find((p) => p.id === preset);
  const submit = (e: FormEvent) => {
    e.preventDefault();
    if (custom && !port.trim()) {
      setErrors({ port: t("inbounds.portRequired") });
      return;
    }
    create.mutate({ preset: preset as Schemas["CreateInboundInputBody"]["preset"], node_id: nodeId, port: port.trim() || undefined, config: custom ? config : undefined });
  };
  return (
    <Drawer
      presentation="modal"
      modalWidth="protocol"
      open={open}
      onOpenChange={onOpenChange}
      title={t("inbounds.newTitle")}
      lead={<span className="creation-icon"><Network size={22} aria-hidden /></span>}
      meta={nodeName ? t("inbounds.newOn", { name: nodeName }) : custom ? t("inbounds.customMeta") : t("inbounds.newMeta")}
      footer={
        <>
          <Button variant="ghost" onClick={() => onOpenChange(false)}>
            {t("common.cancel")}
          </Button>
          <Button variant="primary" type="submit" form="add-inbound" loading={create.isPending} disabled={!chosen}>
            {t("common.add")}
          </Button>
        </>
      }
    >
      <form id="add-inbound" onSubmit={submit} className="pt-5" noValidate>
        <Field label={t("inbounds.protocol")}>
          <QueryBoundary query={presets} pending={<div className="protocol-grid">{[0, 1, 2, 3, 4, 5].map((i) => <Skeleton key={i} style={{ height: 112, borderRadius: 16 }} />)}</div>}>
          {() => <div className="protocol-grid" role="radiogroup" aria-label={t("inbounds.protocol")}>
            {(presets.data ?? []).map((p) => (
              <button key={p.id} type="button" role="radio" aria-checked={preset === p.id} className="opt" onClick={() => setPreset(p.id)}>
                <span className="flex items-start justify-between gap-2 font-semibold"><span>{presetTitle(p)}</span><CircleCheck size={17} className={preset === p.id ? "shrink-0 text-[var(--mirai-600)]" : "shrink-0 text-[var(--ink-200)]"} aria-hidden /></span>
                <span className="text-xs text-[var(--ink-500)]">{presetSummary(p)}</span>
                <LimitBadges clashOnly={p.apps === "mihomo"} shared={!!p.shared} />
              </button>
            ))}
          </div>}
          </QueryBoundary>
        </Field>
        <Field
          label={t("inbounds.portLabel")}
          htmlFor="in-port"
          hint={custom ? t("inbounds.portHintCustom") : t("inbounds.portHint", { port: chosen?.default_port ?? "—", network: chosen?.network ?? "" })}
          error={errors.port}
        >
          <input id="in-port" className="input max-w-[200px]" inputMode="numeric" placeholder={chosen?.default_port} value={port} onChange={(e) => setPort(e.target.value)} aria-invalid={!!errors.port} />
        </Field>
        {chosen && !custom ? <LimitNotes clashOnly={chosen.apps === "mihomo"} shared={!!chosen.shared} /> : null}
        {custom ? (
          <Field label={t("inbounds.configLabel")} hint={t("inbounds.configHint")} error={errors.config}>
            <Editor value={config} onChange={editConfig} invalid={!!errors.config} />
            <div className="mt-2 flex flex-wrap items-center gap-3">
              <Button size="sm" loading={validate.isPending} onClick={() => validate.mutate({ config, node_id: nodeId, port: port.trim() || undefined })}>
                {t("inbounds.validate")}
              </Button>
              <ValidateResult v={validate} />
            </div>
          </Field>
        ) : null}
      </form>
    </Drawer>
  );
}

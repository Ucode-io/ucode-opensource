import { useState } from "react";
import {
  IconArrowDown,
  IconArrowUp,
  IconPencil,
  IconPlus,
  IconTrash,
} from "@tabler/icons-react";
import { useTranslation } from "react-i18next";
import { Button } from "@/shared/ui/button";
import { ConfirmDialog } from "@/shared/ui/confirm-dialog";
import { Icon } from "@/shared/ui/icon";
import { Field, Input } from "@/shared/ui/input";
import { Modal } from "@/shared/ui/modal";
import {
  useCreateEndpoint,
  useDeleteEndpoint,
  useEndpoints,
  useReorderEndpoints,
  useUpdateEndpoint,
  type Endpoint,
} from "../api/endpoints";
import { ENDPOINT_PREFIX, joinPath, splitPath, unmatchedParams } from "../model/endpoint";
import { Empty, SectionHeader, Td, Th, formatDateTime } from "./parts";

/**
 * Свои эндпоинты: короткий публичный адрес вместо длинного внутреннего.
 *
 * Правило разбирается шлюзом посегментно, и выигрывает ПЕРВОЕ совпавшее
 * (`pkg/helper/proxy.go:14`) — поэтому порядок здесь настоящая настройка,
 * а не украшение списка, и его меняют стрелками.
 *
 * Всё остальное про разбор — в `model/endpoint`.
 */
export function EndpointSettings() {
  const { t, i18n } = useTranslation();
  const { endpoints, isLoading } = useEndpoints();
  const remove = useDeleteEndpoint();
  const reorder = useReorderEndpoints();

  const [editing, setEditing] = useState<Endpoint | "new" | null>(null);
  const [deleting, setDeleting] = useState<Endpoint | null>(null);

  /** Переставить соседей местами и отправить весь список: ручка ставит
      `order` по позициям в нём, а не по паре переставленных. */
  const move = (index: number, shift: 1 | -1) => {
    const next = [...endpoints];
    const target = next[index + shift];
    const current = next[index];
    if (!target || !current) return;

    next[index + shift] = current;
    next[index] = target;
    reorder.mutate(next.map((endpoint) => endpoint.id));
  };

  return (
    <div className="flex min-h-0 min-w-0 flex-1 flex-col">
      <SectionHeader title={t("endpoints.title")} hint={t("endpoints.hint")}>
        <Button size="sm" onClick={() => setEditing("new")}>
          <Icon as={IconPlus} size={14} />
          {t("endpoints.create")}
        </Button>
      </SectionHeader>

      <div className="min-h-0 flex-1 overflow-auto">
        <table className="w-full border-separate border-spacing-0">
          <thead>
            <tr>
              <Th className="w-10" />
              <Th>{t("endpoints.from")}</Th>
              <Th>{t("endpoints.to")}</Th>
              <Th className="w-44">{t("endpoints.updated")}</Th>
              <Th className="w-28" />
            </tr>
          </thead>

          <tbody>
            {isLoading && <Empty text={t("common.loading")} colSpan={5} />}
            {!isLoading && !endpoints.length && <Empty text={t("endpoints.empty")} colSpan={5} />}

            {endpoints.map((endpoint, index) => (
              <tr key={endpoint.id} className="group/row hover:bg-surface-hover">
                <Td className="text-center text-2xs text-fg-subtle">{index + 1}</Td>
                <Td className="font-mono text-xs">{endpoint.from}</Td>
                <Td className="font-mono text-xs text-fg-muted">{endpoint.to}</Td>
                <Td className="text-fg-muted">{formatDateTime(endpoint.updatedAt, i18n.language)}</Td>

                <Td className="text-right">
                  <span className="inline-flex gap-1 opacity-0 transition-opacity group-hover/row:opacity-100 focus-within:opacity-100">
                    <RowButton
                      icon={IconArrowUp}
                      label={t("endpoints.moveUp")}
                      disabled={index === 0 || reorder.isPending}
                      onClick={() => move(index, -1)}
                    />
                    <RowButton
                      icon={IconArrowDown}
                      label={t("endpoints.moveDown")}
                      disabled={index === endpoints.length - 1 || reorder.isPending}
                      onClick={() => move(index, 1)}
                    />
                    <RowButton
                      icon={IconPencil}
                      label={t("action.edit")}
                      onClick={() => setEditing(endpoint)}
                    />
                    <RowButton
                      icon={IconTrash}
                      label={t("action.delete")}
                      danger
                      onClick={() => setDeleting(endpoint)}
                    />
                  </span>
                </Td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {editing && (
        <EndpointDialog
          endpoint={editing === "new" ? null : editing}
          count={endpoints.length}
          onClose={() => setEditing(null)}
        />
      )}

      {deleting && (
        <ConfirmDialog
          title={t("endpoints.deleteTitle", { path: deleting.from })}
          description={t("endpoints.deleteDescription")}
          confirmLabel={t("action.delete")}
          busy={remove.isPending}
          onClose={() => setDeleting(null)}
          onConfirm={() => remove.mutate(deleting.id, { onSuccess: () => setDeleting(null) })}
        />
      )}
    </div>
  );
}

function RowButton({
  icon,
  label,
  danger = false,
  disabled = false,
  onClick,
}: {
  icon: typeof IconPencil;
  label: string;
  danger?: boolean;
  disabled?: boolean;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled}
      aria-label={label}
      title={label}
      className={`grid size-7 place-items-center rounded-md text-fg-subtle transition-colors disabled:pointer-events-none disabled:opacity-30 ${
        danger ? "hover:bg-danger-subtle hover:text-danger" : "hover:bg-surface-active hover:text-fg"
      }`}
    >
      <Icon as={icon} size={14} />
    </button>
  );
}

/**
 * Правило: откуда и куда.
 *
 * Приставка `/x-api/` показана, но не правится: другие пути через
 * подмену не проходят вовсе (`api.go:37`), и поле, в котором её можно
 * стереть, обещало бы работающее правило там, где его не будет.
 *
 * Имя подстановки, которого нет в «откуда», ловится до сохранения:
 * шлюз оставил бы его в адресе как есть, и запрос молча вернул бы 404.
 */
function EndpointDialog({
  endpoint,
  count,
  onClose,
}: {
  endpoint: Endpoint | null;
  count: number;
  onClose: () => void;
}) {
  const { t } = useTranslation();
  const create = useCreateEndpoint(count);
  const update = useUpdateEndpoint();

  const [from, setFrom] = useState(
    endpoint ? splitPath(ENDPOINT_PREFIX, endpoint.from) : "",
  );
  const [to, setTo] = useState(endpoint ? splitPath("/", endpoint.to) : "");

  const fromPath = joinPath(ENDPOINT_PREFIX, from);
  const toPath = joinPath("/", to);
  const unmatched = unmatchedParams(fromPath, toPath);
  const busy = create.isPending || update.isPending;

  return (
    <Modal onClose={onClose}>
      <form
        className="flex w-full max-w-lg flex-col gap-4 rounded-xl border border-border bg-surface p-5 shadow-modal"
        onSubmit={(event) => {
          event.preventDefault();
          const draft = { from: fromPath, to: toPath };

          if (endpoint) update.mutate({ id: endpoint.id, draft }, { onSuccess: onClose });
          else create.mutate(draft, { onSuccess: onClose });
        }}
      >
        <div>
          <h2 className="text-base font-semibold">
            {endpoint ? t("endpoints.editTitle") : t("endpoints.create")}
          </h2>
          <p className="mt-1 text-xs text-fg-subtle">{t("endpoints.formHint")}</p>
        </div>

        <Field label={t("endpoints.from")}>
          <div className="flex items-center gap-1">
            <span className="shrink-0 font-mono text-xs text-fg-subtle">{ENDPOINT_PREFIX}</span>
            <Input
              autoFocus
              required
              value={from}
              onChange={(event) => setFrom(event.target.value)}
              placeholder="order/{id}"
              className="font-mono text-xs"
            />
          </div>
        </Field>

        <Field label={t("endpoints.to")}>
          <div className="flex items-center gap-1">
            <span className="shrink-0 font-mono text-xs text-fg-subtle">/</span>
            <Input
              required
              value={to}
              onChange={(event) => setTo(event.target.value)}
              placeholder="v1/object/order/{id}"
              className="font-mono text-xs"
            />
          </div>
        </Field>

        {unmatched.length > 0 && (
          <p className="rounded-md bg-danger-subtle px-3 py-2 text-xs text-danger">
            {t("endpoints.unmatched", { names: unmatched.map((name) => `{${name}}`).join(", ") })}
          </p>
        )}

        <div className="flex justify-end gap-2">
          <Button type="button" variant="ghost" onClick={onClose}>
            {t("action.cancel")}
          </Button>
          <Button type="submit" disabled={busy || !from.trim() || !to.trim() || unmatched.length > 0}>
            {busy ? t("common.saving") : endpoint ? t("action.save") : t("action.create")}
          </Button>
        </div>
      </form>
    </Modal>
  );
}

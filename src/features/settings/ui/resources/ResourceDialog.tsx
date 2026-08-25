import { useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/shared/ui/button";
import { Field, Input } from "@/shared/ui/input";
import { Modal } from "@/shared/ui/modal";
import {
  emptyDraft,
  toDraft,
  useCreateResource,
  useReconnectResource,
  useResource,
  useUpdateResource,
  RESOURCE_LABELS,
  RESOURCE_SPECS,
  type ResourceDraft,
} from "../../api/resources";
import { ResourceFields } from "./ResourceFields";
import { ResourceIcon } from "./ResourceIcon";
import { ResourceVariables } from "./ResourceVariables";

/**
 * Создание ресурса.
 *
 * Тип уже выбран (`ResourceTypePicker`) и здесь не меняется: сменить его
 * у существующей строки всё равно нельзя — UPDATE его не трогает, —
 * и выпадающий список в форме правки обещал бы то, чего не будет.
 */
export function CreateResourceDialog({ kind, onClose }: { kind: string; onClose: () => void }) {
  const { t } = useTranslation();
  const create = useCreateResource();

  const spec = RESOURCE_SPECS[kind];
  const [draft, setDraft] = useState<ResourceDraft>(() => {
    const empty = emptyDraft(kind);
    /* REST без переменных бесполезен: своих настроек у него нет вовсе. */
    return spec?.variables ? { ...empty, variables: [{ id: "", key: "", value: "" }] } : empty;
  });

  return (
    <ResourceForm
      title={`${t("resources.create")} · ${RESOURCE_LABELS[kind] ?? kind}`}
      submitLabel={t("action.create")}
      kind={kind}
      draft={draft}
      onChange={setDraft}
      busy={create.isPending}
      /* Учётные данные таких типов бэкенд выдаёт сам при создании —
         заполнять нечего, поля появятся уже в правке. */
      note={spec?.readOnly ? t("resources.provisioned") : ""}
      hideFields={Boolean(spec?.readOnly)}
      onSubmit={() => create.mutate({ kind, draft }, { onSuccess: onClose })}
      onClose={onClose}
    />
  );
}

/**
 * Правка ресурса.
 *
 * Форма читает ответ `GET /v2/company/project/resource/{id}`, а не строку
 * списка: переменных в списке нет, а у строк, заведённых вместе с
 * проектом, нет и настроек.
 */
export function EditResourceDialog({ id, onClose }: { id: string; onClose: () => void }) {
  const { t } = useTranslation();
  const { resource, isLoading } = useResource(id);
  const update = useUpdateResource();
  const reconnect = useReconnectResource();

  const [draft, setDraft] = useState<ResourceDraft | null>(null);
  const current = draft ?? (resource ? toDraft(resource) : null);

  if (isLoading || !resource || !current) {
    return (
      <Modal onClose={onClose}>
        <div className="rounded-xl border border-border bg-surface p-5 text-sm text-fg-muted shadow-modal">
          {t("common.loading")}
        </div>
      </Modal>
    );
  }

  const spec = RESOURCE_SPECS[resource.kind];
  /*
   * Такой ресурс завела не эта ручка, и правит его не она: строку
   * проекта UPDATE не найдёт и промолчит, а Telegram шлюз отклонит.
   *
   * Незнакомый тип — сюда же: его настроек мы не знаем, а PUT кладёт
   * `settings` целиком, то есть сохранение стёрло бы их подчистую.
   */
  const managed = !spec || Boolean(spec.managed);

  return (
    <ResourceForm
      title={`${t("resources.editTitle")} · ${RESOURCE_LABELS[resource.kind] ?? resource.kind}`}
      submitLabel={t("action.save")}
      kind={resource.kind}
      draft={current}
      onChange={setDraft}
      busy={update.isPending}
      readOnly={managed}
      fieldsReadOnly={Boolean(spec?.readOnly)}
      note={
        spec?.system
          ? t("resources.system")
          : managed
            ? t("resources.managed")
            : spec?.readOnly
              ? t("resources.readOnly")
              : ""
      }
      onSubmit={() => update.mutate({ resource, draft: current }, { onSuccess: onClose })}
      onClose={onClose}
      /* Переподключение — не сохранение: оно ничего не меняет в строке,
         а заново раздаёт службам учётку базы из хранилища. Поэтому
         отдельной кнопкой у левого края, а не рядом с «Сохранить». */
      extra={
        spec?.reconnect ? (
          <Button
            type="button"
            variant="secondary"
            disabled={reconnect.isPending}
            onClick={() => reconnect.mutate(resource.id)}
          >
            {reconnect.isPending ? t("resources.reconnecting") : t("resources.reconnect")}
          </Button>
        ) : null
      }
    />
  );
}

/**
 * Общая форма: имя, поля типа, переменные.
 *
 * Одна на создание и правку, потому что отличаются они ровно двумя
 * подписями и тем, откуда взялся черновик. Две копии этой разметки
 * разъехались бы на первой же новой настройке.
 */
function ResourceForm({
  title,
  submitLabel,
  kind,
  draft,
  onChange,
  busy,
  note,
  readOnly = false,
  fieldsReadOnly = false,
  hideFields = false,
  extra = null,
  onSubmit,
  onClose,
}: {
  title: string;
  submitLabel: string;
  kind: string;
  draft: ResourceDraft;
  onChange: (draft: ResourceDraft) => void;
  busy: boolean;
  /** Одна строка о том, почему поля такие. Пусто — сказать нечего. */
  note: string;
  /** Ничего не правится: ресурс живёт своей ручкой. */
  readOnly?: boolean;
  /** Имя правится, настройки — нет: их выдал бэкенд. */
  fieldsReadOnly?: boolean;
  /** Настроек ещё нет: их выдадут при создании. */
  hideFields?: boolean;
  /** Действие, которое не сохраняет форму. Встаёт у левого края низа. */
  extra?: ReactNode;
  onSubmit: () => void;
  onClose: () => void;
}) {
  const { t } = useTranslation();
  const spec = RESOURCE_SPECS[kind];

  return (
    <Modal onClose={onClose}>
      <form
        className="flex max-h-[85vh] w-full max-w-xl flex-col gap-4 overflow-y-auto rounded-xl border border-border bg-surface p-5 shadow-modal"
        onSubmit={(event) => {
          event.preventDefault();
          if (!readOnly) onSubmit();
        }}
      >
        <h2 className="flex items-center gap-2 text-base font-semibold">
          <span className="text-fg-muted">
            <ResourceIcon kind={kind} />
          </span>
          {title}
        </h2>

        {note && (
          <p className="rounded-md bg-surface-hover px-3 py-2 text-xs text-fg-muted">{note}</p>
        )}

        <Field label={t("settings.name")}>
          <Input
            autoFocus={!readOnly}
            required
            value={draft.name}
            readOnly={readOnly}
            onChange={(event) => onChange({ ...draft, name: event.target.value })}
          />
        </Field>

        {!hideFields && (
          <ResourceFields
            kind={kind}
            values={draft.settings}
            readOnly={readOnly || fieldsReadOnly}
            onChange={(settings) => onChange({ ...draft, settings })}
          />
        )}

        {spec?.variables && !readOnly && (
          <ResourceVariables
            variables={draft.variables}
            onChange={(variables) => onChange({ ...draft, variables })}
          />
        )}

        <div className="flex items-center justify-end gap-2">
          {extra && <span className="mr-auto">{extra}</span>}

          <Button type="button" variant="ghost" onClick={onClose}>
            {t(readOnly ? "action.close" : "action.cancel")}
          </Button>

          {!readOnly && (
            <Button type="submit" disabled={busy || !draft.name.trim()}>
              {busy ? t("common.saving") : submitLabel}
            </Button>
          )}
        </div>
      </form>
    </Modal>
  );
}

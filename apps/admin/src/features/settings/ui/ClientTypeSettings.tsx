import { useState } from "react";
import { IconPencil, IconPlus, IconTable, IconTrash } from "@tabler/icons-react";
import { useTranslation } from "react-i18next";
import { localized, useTableFields, useTables } from "@/features/table";
import { useDataLanguages } from "@/features/workspace";
import { Button } from "@/shared/ui/button";
import { Checkbox } from "@/shared/ui/checkbox";
import { ConfirmDialog } from "@/shared/ui/confirm-dialog";
import { Icon } from "@/shared/ui/icon";
import { Dropdown } from "@/shared/ui/dropdown";
import { Field, Input } from "@/shared/ui/input";
import { Modal } from "@/shared/ui/modal";
import {
  DEFAULT_SESSION_LIMIT,
  useClientType,
  useClientTypes,
  useCreateClientType,
  useDeleteClientType,
  useLoginTables,
  useUpdateClientType,
  type ClientType,
  type ClientTypeDraft,
} from "../api/client-types";
import {
  useClientConnections,
  useDeleteClientConnection,
  useSaveClientConnection,
  type ClientConnection,
  type ClientConnectionDraft,
} from "../api/client-connections";
import { Empty, SectionHeader, Td, Th } from "./parts";

/**
 * Типы клиентов — аудитории проекта.
 *
 * Тип клиента — это ответ на вопрос «кто входит»: у него своя таблица
 * входа, свои роли и свои люди. Всё, что рядом в этих настройках,
 * висит на нём: роль заводится под тип, список людей приходит по типу,
 * API-ключ выдаётся под тип. Поэтому раздел стоит перед ними, а не
 * прячется внутри списка людей.
 *
 * В старой админке он жил тем же окном настроек — кнопкой «Add client
 * type» под списком прав (`SettingsPopup.jsx:146`) и карандашом рядом
 * с выбранным типом.
 */
export function ClientTypeSettings() {
  const { t } = useTranslation();
  const { clientTypes, isLoading } = useClientTypes();
  const { loginTables } = useLoginTables();
  const remove = useDeleteClientType();

  /** `null` — форма закрыта, `"new"` — новый тип, иначе правка. */
  const [editing, setEditing] = useState<ClientType | "new" | null>(null);
  const [deleting, setDeleting] = useState<ClientType | null>(null);
  /** Открытые «таблицы аудитории»: чей список сейчас правят. */
  const [connecting, setConnecting] = useState<ClientType | null>(null);

  const tableLabel = (slug: string) =>
    loginTables.find((table) => table.slug === slug)?.label || slug;

  return (
    <div className="flex min-h-0 min-w-0 flex-1 flex-col">
      <SectionHeader title={t("clientTypes.title")} hint={t("clientTypes.hint")}>
        <Button size="sm" onClick={() => setEditing("new")}>
          <Icon as={IconPlus} size={14} />
          {t("clientTypes.create")}
        </Button>
      </SectionHeader>

      <div className="min-h-0 flex-1 overflow-auto">
        <table className="w-full border-separate border-spacing-0">
          <thead>
            <tr>
              <Th>{t("settings.name")}</Th>
              <Th>{t("tableSettings.loginTable")}</Th>
              <Th className="w-32">{t("clientTypes.sessionLimit")}</Th>
              <Th>{t("clientTypes.selfRegister")}</Th>
              <Th className="w-40">{t("clientConnections.title")}</Th>
              <Th className="w-20" />
            </tr>
          </thead>

          <tbody>
            {isLoading && <Empty text={t("common.loading")} colSpan={6} />}
            {!isLoading && !clientTypes.length && (
              <Empty text={t("clientTypes.empty")} colSpan={6} />
            )}

            {clientTypes.map((clientType) => (
              <tr key={clientType.id} className="group/row hover:bg-surface-hover">
                <Td>{clientType.name}</Td>

                <Td className="font-mono text-xs text-fg-muted">
                  {tableLabel(clientType.tableSlug) || "—"}
                </Td>

                <Td className="text-fg-muted">{clientType.sessionLimit}</Td>

                <Td className="text-fg-muted">
                  {clientType.selfRegister ? t("action.yes") : t("action.no")}
                </Td>

                <Td>
                  <button
                    type="button"
                    onClick={() => setConnecting(clientType)}
                    className="inline-flex h-7 items-center gap-1.5 rounded-md px-2 text-xs text-fg-muted transition-colors hover:bg-surface-active hover:text-fg"
                  >
                    <Icon as={IconTable} size={14} />
                    {t("clientConnections.open")}
                  </button>
                </Td>

                <Td className="text-right">
                  <span className="inline-flex gap-1 opacity-0 transition-opacity group-hover/row:opacity-100 focus-within:opacity-100">
                    <button
                      type="button"
                      onClick={() => setEditing(clientType)}
                      aria-label={t("action.edit")}
                      title={t("action.edit")}
                      className="grid size-7 place-items-center rounded-md text-fg-subtle transition-colors hover:bg-surface-active hover:text-fg"
                    >
                      <Icon as={IconPencil} size={14} />
                    </button>

                    <button
                      type="button"
                      onClick={() => setDeleting(clientType)}
                      aria-label={t("action.delete")}
                      title={t("action.delete")}
                      className="grid size-7 place-items-center rounded-md text-fg-subtle transition-colors hover:bg-danger-subtle hover:text-danger"
                    >
                      <Icon as={IconTrash} size={14} />
                    </button>
                  </span>
                </Td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {editing === "new" && <CreateDialog onClose={() => setEditing(null)} />}

      {editing && editing !== "new" && (
        <EditDialog id={editing.id} onClose={() => setEditing(null)} />
      )}

      {connecting && (
        <ConnectionsDialog clientType={connecting} onClose={() => setConnecting(null)} />
      )}

      {deleting && (
        <ConfirmDialog
          title={t("clientTypes.deleteTitle", { name: deleting.name })}
          description={t("clientTypes.deleteDescription")}
          confirmLabel={t("action.delete")}
          busy={remove.isPending}
          onClose={() => setDeleting(null)}
          onConfirm={() => remove.mutate(deleting.id, { onSuccess: () => setDeleting(null) })}
        />
      )}
    </div>
  );
}

/**
 * Новый тип клиента.
 *
 * Таблица входа необязательна: без неё бэкенд заведёт свою, `<имя>_users`,
 * и пометит её таблицей входа. Так и заводят первую аудиторию — таблицы
 * для неё ещё нет.
 *
 * «Адрес после входа» здесь нет намеренно: создание его не сохраняет
 * (см. api/client-types), и поле, которое молча теряет введённое, мы
 * не рисуем. Оно есть в правке.
 */
function CreateDialog({ onClose }: { onClose: () => void }) {
  const { t } = useTranslation();
  const create = useCreateClientType();

  const [draft, setDraft] = useState<ClientTypeDraft>({
    name: "",
    tableSlug: "",
    sessionLimit: DEFAULT_SESSION_LIMIT,
    selfRegister: false,
    selfRecover: false,
    defaultPage: "",
  });

  return (
    <ClientTypeForm
      title={t("clientTypes.create")}
      submitLabel={t("action.create")}
      draft={draft}
      onChange={setDraft}
      busy={create.isPending}
      onSubmit={() => create.mutate(draft, { onSuccess: onClose })}
      onClose={onClose}
      creating
    />
  );
}

/**
 * Правка типа клиента.
 *
 * Форма ждёт ответа `GET /v2/client-type/{id}`, а не берёт строку
 * списка: список на mongo-проектах приходит урезанным, и сохранение
 * поверх него обнулило бы предел сессий (см. api/client-types).
 *
 * «Самовосстановления» здесь нет: правка его не сохраняет — шлюз
 * шлёт его под другим именем (docs/backend-notes.md). Задать его
 * можно только при создании.
 */
function EditDialog({ id, onClose }: { id: string; onClose: () => void }) {
  const { t } = useTranslation();
  const { clientType, isLoading } = useClientType(id);
  const update = useUpdateClientType();

  const [draft, setDraft] = useState<ClientTypeDraft | null>(null);
  const current = draft ?? (clientType ? toDraft(clientType) : null);

  if (isLoading || !clientType || !current) {
    return (
      <Modal onClose={onClose}>
        <div className="rounded-xl border border-border bg-surface p-5 text-sm text-fg-muted shadow-modal">
          {t("common.loading")}
        </div>
      </Modal>
    );
  }

  return (
    <ClientTypeForm
      title={t("clientTypes.editTitle")}
      submitLabel={t("action.save")}
      draft={current}
      onChange={setDraft}
      busy={update.isPending}
      onSubmit={() => update.mutate({ clientType, draft: current }, { onSuccess: onClose })}
      onClose={onClose}
    />
  );
}

function toDraft(clientType: ClientType): ClientTypeDraft {
  return {
    name: clientType.name,
    tableSlug: clientType.tableSlug,
    sessionLimit: clientType.sessionLimit,
    selfRegister: clientType.selfRegister,
    selfRecover: clientType.selfRecover,
    defaultPage: clientType.defaultPage,
  };
}

/**
 * Поля типа клиента. Набор у создания и у правки разный — и не по
 * вкусу, а по тому, что доезжает до базы: создание теряет адрес после
 * входа, правка теряет самовосстановление.
 */
function ClientTypeForm({
  title,
  submitLabel,
  draft,
  onChange,
  busy,
  onSubmit,
  onClose,
  creating = false,
}: {
  title: string;
  submitLabel: string;
  draft: ClientTypeDraft;
  onChange: (draft: ClientTypeDraft) => void;
  busy: boolean;
  onSubmit: () => void;
  onClose: () => void;
  creating?: boolean;
}) {
  const { t } = useTranslation();
  const { loginTables } = useLoginTables();

  const put = (patch: Partial<ClientTypeDraft>) => onChange({ ...draft, ...patch });

  return (
    <Modal onClose={onClose}>
      <form
        className="flex w-full max-w-md flex-col gap-4 rounded-xl border border-border bg-surface p-5 shadow-modal"
        onSubmit={(event) => {
          event.preventDefault();
          onSubmit();
        }}
      >
        <h2 className="text-base font-semibold">{title}</h2>

        <Field label={t("settings.name")} hint={t("clientTypes.nameHint")}>
          <Input
            autoFocus
            required
            value={draft.name}
            onChange={(event) => put({ name: event.target.value })}
          />
        </Field>

        <Field label={t("tableSettings.loginTable")} hint={t("clientTypes.tableHint")}>
          <Dropdown
            value={draft.tableSlug}
            placeholder={creating ? t("clientTypes.tableAuto") : "—"}
            items={loginTables.map((table) => ({ value: table.slug, label: table.label }))}
            onChange={(tableSlug) => put({ tableSlug })}
          />
        </Field>

        <Field label={t("clientTypes.sessionLimit")} hint={t("clientTypes.sessionLimitHint")}>
          {/* Нативное числовое поле: шаг, стрелки и проверка ввода —
              бесплатно. Ноль бэкенд принял бы и записал, а вошедших
              под этим типом стало бы ноль. */}
          <Input
            type="number"
            min={1}
            required
            value={draft.sessionLimit}
            onChange={(event) => put({ sessionLimit: Number(event.target.value) })}
          />
        </Field>

        {!creating && (
          <Field label={t("clientTypes.defaultPage")} hint={t("clientTypes.defaultPageHint")}>
            <Input
              value={draft.defaultPage}
              onChange={(event) => put({ defaultPage: event.target.value })}
            />
          </Field>
        )}

        <label className="flex items-start gap-2">
          <Checkbox
            className="mt-0.5"
            checked={draft.selfRegister}
            onChange={(event) => put({ selfRegister: event.target.checked })}
          />
          <span className="min-w-0">
            <span className="block text-sm text-fg">{t("clientTypes.selfRegister")}</span>
            <span className="block text-xs text-fg-subtle">
              {t("clientTypes.selfRegisterHint")}
            </span>
          </span>
        </label>

        {/* Самовосстановление задаётся только при создании: правку его
            бэкенд теряет (docs/backend-notes.md). */}
        {creating && (
          <label className="flex items-start gap-2">
            <Checkbox
              className="mt-0.5"
              checked={draft.selfRecover}
              onChange={(event) => put({ selfRecover: event.target.checked })}
            />
            <span className="min-w-0">
              <span className="block text-sm text-fg">{t("clientTypes.selfRecover")}</span>
              <span className="block text-xs text-fg-subtle">
                {t("clientTypes.selfRecoverHint")}
              </span>
            </span>
          </label>
        )}

        <div className="flex justify-end gap-2">
          <Button type="button" variant="ghost" onClick={onClose}>
            {t("action.cancel")}
          </Button>
          <Button type="submit" disabled={!draft.name.trim() || busy}>
            {busy ? t("common.saving") : submitLabel}
          </Button>
        </div>
      </form>
    </Modal>
  );
}

/**
 * Таблицы аудитории: что приложение заказчика показывает этому типу
 * клиента.
 *
 * Список уезжает приложению вместе с ответом на вход — это его меню,
 * а не наше. Поэтому здесь только то, что оттуда и читают: имя таблицы
 * так, как её видит пользователь, сама таблица и поле таблицы входа,
 * по которому находится «его» строка.
 *
 * Чего нет: «главной таблицы» — её не читает ни одна ручка; вида,
 * значка и подписи вида — их не пишет ни одна ручка. Подробности
 * в docs/PARITY.md.
 */
function ConnectionsDialog({
  clientType,
  onClose,
}: {
  clientType: ClientType;
  onClose: () => void;
}) {
  const { t } = useTranslation();
  const { connections, isLoading } = useClientConnections(clientType.id);
  const remove = useDeleteClientConnection(clientType.id);

  /** `null` — показываем список, иначе форму: одна модалка на оба вида. */
  const [editing, setEditing] = useState<ClientConnection | "new" | null>(null);
  const [deleting, setDeleting] = useState<ClientConnection | null>(null);

  const { items: tables } = useTables("");
  const { current: language } = useDataLanguages();

  const tableLabel = (slug: string) => {
    const table = tables.find((item) => item.slug === slug);
    return table ? localized(table.labels, language, table.label) : slug;
  };

  return (
    <Modal onClose={onClose}>
      <div className="flex max-h-[85vh] w-full max-w-2xl flex-col rounded-xl border border-border bg-surface shadow-modal">
        <header className="flex shrink-0 items-start gap-3 border-b border-border px-5 py-3">
          <div className="min-w-0 flex-1">
            <h2 className="text-base font-semibold">{t("clientConnections.title")}</h2>
            <p className="mt-0.5 text-xs text-fg-subtle">
              {t("clientConnections.hint", { name: clientType.name })}
            </p>
          </div>

          {!editing && (
            <Button size="sm" onClick={() => setEditing("new")}>
              <Icon as={IconPlus} size={14} />
              {t("action.create")}
            </Button>
          )}
        </header>

        {editing ? (
          <ConnectionForm
            clientType={clientType}
            connection={editing === "new" ? null : editing}
            onClose={() => setEditing(null)}
          />
        ) : (
          <div className="min-h-0 flex-1 overflow-auto">
            <table className="w-full border-separate border-spacing-0">
              <thead>
                <tr>
                  <Th>{t("settings.name")}</Th>
                  <Th>{t("clientConnections.table")}</Th>
                  <Th>{t("clientConnections.field")}</Th>
                  <Th className="w-20" />
                </tr>
              </thead>

              <tbody>
                {isLoading && <Empty text={t("common.loading")} colSpan={4} />}
                {!isLoading && !connections.length && (
                  <Empty text={t("clientConnections.empty")} colSpan={4} />
                )}

                {connections.map((connection) => (
                  <tr key={connection.id} className="group/row hover:bg-surface-hover">
                    <Td>{connection.name || "—"}</Td>
                    <Td className="text-fg-muted">{tableLabel(connection.tableSlug)}</Td>
                    <Td className="font-mono text-xs text-fg-muted">
                      {connection.fieldSlug || "—"}
                    </Td>

                    <Td className="text-right">
                      <span className="inline-flex gap-1 opacity-0 transition-opacity group-hover/row:opacity-100 focus-within:opacity-100">
                        <button
                          type="button"
                          onClick={() => setEditing(connection)}
                          aria-label={t("action.edit")}
                          title={t("action.edit")}
                          className="grid size-7 place-items-center rounded-md text-fg-subtle transition-colors hover:bg-surface-active hover:text-fg"
                        >
                          <Icon as={IconPencil} size={14} />
                        </button>

                        <button
                          type="button"
                          onClick={() => setDeleting(connection)}
                          aria-label={t("action.delete")}
                          title={t("action.delete")}
                          className="grid size-7 place-items-center rounded-md text-fg-subtle transition-colors hover:bg-danger-subtle hover:text-danger"
                        >
                          <Icon as={IconTrash} size={14} />
                        </button>
                      </span>
                    </Td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}

        {!editing && (
          <footer className="flex h-12 shrink-0 items-center justify-end border-t border-border px-5">
            <Button variant="ghost" onClick={onClose}>
              {t("action.close")}
            </Button>
          </footer>
        )}
      </div>

      {deleting && (
        <ConfirmDialog
          title={t("clientConnections.deleteTitle", { name: deleting.name || deleting.tableSlug })}
          description={t("clientConnections.deleteDescription")}
          confirmLabel={t("action.delete")}
          busy={remove.isPending}
          onClose={() => setDeleting(null)}
          onConfirm={() => remove.mutate(deleting.id, { onSuccess: () => setDeleting(null) })}
        />
      )}
    </Modal>
  );
}

/**
 * Одна связка. Поле выбирается из полей ТАБЛИЦЫ ВХОДА, а не выбранной
 * таблицы: ручка ищет строку так — берёт значение этого поля у строки
 * вошедшего и по нему достаёт запись (`login.go:401`).
 *
 * Поле необязательно: без него ручка пробует поле `<таблица>_id`,
 * а если и его нет — связка остаётся просто пунктом меню приложения.
 */
function ConnectionForm({
  clientType,
  connection,
  onClose,
}: {
  clientType: ClientType;
  connection: ClientConnection | null;
  onClose: () => void;
}) {
  const { t } = useTranslation();
  const save = useSaveClientConnection(clientType.id);

  const [search, setSearch] = useState("");
  const { items: tables, isLoading: loadingTables } = useTables(search);
  const { items: loginFields } = useTableFields(clientType.tableSlug, "");
  const { current: language } = useDataLanguages();

  const [draft, setDraft] = useState<ClientConnectionDraft>({
    name: connection?.name ?? "",
    tableSlug: connection?.tableSlug ?? "",
    fieldSlug: connection?.fieldSlug ?? "",
  });

  const put = (patch: Partial<ClientConnectionDraft>) =>
    setDraft((current) => ({ ...current, ...patch }));

  return (
    <form
      className="flex min-h-0 flex-1 flex-col"
      onSubmit={(event) => {
        event.preventDefault();
        save.mutate({ id: connection?.id ?? "", draft }, { onSuccess: onClose });
      }}
    >
      <div className="min-h-0 flex-1 space-y-4 overflow-y-auto p-5">
        <Field label={t("settings.name")} hint={t("clientConnections.nameHint")}>
          <Input
            autoFocus
            required
            value={draft.name}
            onChange={(event) => put({ name: event.target.value })}
          />
        </Field>

        <Field label={t("clientConnections.table")}>
          {/* Поиск отдан серверу: таблиц в живом проекте сотни, и список
              выбора без него — это стена из ста пятидесяти строк. Поле
              поиска живёт внутри списка: искать вслепую, не видя строк,
              незачем. */}
          <Dropdown
            value={draft.tableSlug}
            placeholder="—"
            items={tables.map((table) => ({
              value: table.slug,
              label: localized(table.labels, language, table.label),
            }))}
            search={search}
            searchPlaceholder={t("menuForm.tableSearch")}
            emptyText={t("menuForm.tableEmpty")}
            loading={loadingTables}
            onSearch={setSearch}
            onChange={(tableSlug) => put({ tableSlug })}
          />
        </Field>

        <Field label={t("clientConnections.field")} hint={t("clientConnections.fieldHint")}>
          <Dropdown
            value={draft.fieldSlug}
            placeholder="—"
            items={loginFields.map((field) => ({ value: field.slug, label: field.label }))}
            onChange={(fieldSlug) => put({ fieldSlug })}
          />
        </Field>
      </div>

      <footer className="flex h-12 shrink-0 items-center justify-end gap-2 border-t border-border px-5">
        <Button type="button" variant="ghost" onClick={onClose}>
          {t("action.cancel")}
        </Button>
        <Button type="submit" disabled={!draft.name.trim() || !draft.tableSlug || save.isPending}>
          {save.isPending ? t("common.saving") : t("action.save")}
        </Button>
      </footer>
    </form>
  );
}

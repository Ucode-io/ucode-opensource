import { useDeferredValue, useState } from "react";
import {
  IconCopy,
  IconEye,
  IconEyeOff,
  IconPencil,
  IconPlus,
  IconTrash,
} from "@tabler/icons-react";
import { useTranslation } from "react-i18next";
import { toast } from "@/shared/lib/toast";
import { Button } from "@/shared/ui/button";
import { Checkbox } from "@/shared/ui/checkbox";
import { ConfirmDialog } from "@/shared/ui/confirm-dialog";
import { Icon } from "@/shared/ui/icon";
import { Field, Input, Select } from "@/shared/ui/input";
import { Modal } from "@/shared/ui/modal";
import {
  API_KEYS_PAGE,
  useApiKeys,
  useClientPlatforms,
  useCreateApiKey,
  useDeleteApiKey,
  useUpdateApiKey,
  type ApiKey,
  type ApiKeyDraft,
} from "../api/api-keys";
import { useClientTypes, useRoles } from "../api/roles";
import { Empty, Pager, SectionHeader, Td, Th, formatDateTime } from "./parts";

/**
 * API-ключи: чем чужая программа входит вместо человека.
 *
 * Ключ — это пара «логин + секрет» с ролью: что разрешено ключу,
 * решают права ЕГО роли, те же самые, что и у людей. Отдельного набора
 * прав у ключа нет, и это правильно — иначе права раздавались бы
 * в двух местах.
 *
 * Ключ живёт в ОДНОМ окружении, том, в котором его завели: об этом
 * сказано прямо в подзаголовке, потому что из списка это никак
 * не видно, а ключ из dev в prod просто не работает.
 */
export function ApiKeySettings() {
  const { t, i18n } = useTranslation();

  const [query, setQuery] = useState("");
  const search = useDeferredValue(query);
  const [page, setPage] = useState(1);

  const { apiKeys, count, isLoading } = useApiKeys({ search, page });
  const { roles } = useRoles();
  const { clientTypes } = useClientTypes();
  const remove = useDeleteApiKey();

  const [editing, setEditing] = useState<ApiKey | "new" | null>(null);
  const [deleting, setDeleting] = useState<ApiKey | null>(null);

  const roleName = (id: string) => roles.find((role) => role.id === id)?.name ?? "";

  return (
    <div className="flex min-h-0 min-w-0 flex-1 flex-col">
      <SectionHeader title={t("apiKeys.title")} hint={t("apiKeys.hint")}>
        {/* Ширину задаёт обёртка — см. UserSettings. */}
        <div className="w-48">
          <Input
            value={query}
            onChange={(event) => {
              setQuery(event.target.value);
              setPage(1);
            }}
            placeholder={t("apiKeys.search")}
            aria-label={t("apiKeys.search")}
            className="h-7 text-sm"
          />
        </div>

        <Button size="sm" onClick={() => setEditing("new")}>
          <Icon as={IconPlus} size={14} />
          {t("apiKeys.create")}
        </Button>
      </SectionHeader>

      <div className="min-h-0 flex-1 overflow-auto">
        <table className="w-full border-separate border-spacing-0">
          <thead>
            <tr>
              <Th>{t("apiKeys.name")}</Th>
              <Th>{t("apiKeys.appId")}</Th>
              <Th>{t("apiKeys.appSecret")}</Th>
              <Th>{t("settings.role")}</Th>
              <Th>{t("apiKeys.createdAt")}</Th>
              <Th className="w-20" />
            </tr>
          </thead>

          <tbody>
            {isLoading && <Empty text={t("common.loading")} colSpan={6} />}
            {!isLoading && !apiKeys.length && <Empty text={t("apiKeys.empty")} colSpan={6} />}

            {apiKeys.map((key) => (
              <tr key={key.id} className="group/row hover:bg-surface-hover">
                <Td>
                  <span className="flex items-center gap-2">
                    {key.name}
                    {!key.active && (
                      <span className="rounded bg-surface-active px-1.5 py-0.5 text-2xs text-fg-subtle">
                        {t("apiKeys.disabled")}
                      </span>
                    )}
                  </span>
                </Td>

                <Td>
                  <Secret value={key.appId} />
                </Td>

                <Td>
                  <Secret value={key.appSecret} masked />
                </Td>

                <Td className="text-fg-muted">{roleName(key.roleId)}</Td>
                <Td className="text-fg-muted">{formatDateTime(key.createdAt, i18n.language)}</Td>

                <Td className="text-right">
                  <span className="inline-flex gap-1 opacity-0 transition-opacity group-hover/row:opacity-100 focus-within:opacity-100">
                    <button
                      type="button"
                      onClick={() => setEditing(key)}
                      aria-label={t("action.edit")}
                      title={t("action.edit")}
                      className="grid size-7 place-items-center rounded-md text-fg-subtle transition-colors hover:bg-surface-active hover:text-fg"
                    >
                      <Icon as={IconPencil} size={14} />
                    </button>

                    <button
                      type="button"
                      onClick={() => setDeleting(key)}
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

      <Pager page={page} total={count} limit={API_KEYS_PAGE} onPage={setPage} />

      {editing && (
        <ApiKeyDialog
          apiKey={editing === "new" ? null : editing}
          roles={roles}
          clientTypes={clientTypes}
          onClose={() => setEditing(null)}
        />
      )}

      {deleting && (
        <ConfirmDialog
          title={t("apiKeys.deleteTitle", { name: deleting.name })}
          description={t("apiKeys.deleteDescription")}
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
 * Значение, которое копируют, а не читают.
 *
 * Секрет спрятан по умолчанию: он приезжает в ответе списка, и открытая
 * настройка означала бы пароль на экране у всякого, кто зашёл посмотреть
 * ключи. Кнопка копирования при этом работает и не показывая — так его
 * обычно и берут.
 */
function Secret({ value, masked = false }: { value: string; masked?: boolean }) {
  const { t } = useTranslation();
  const [shown, setShown] = useState(false);

  if (!value) return <span className="text-fg-subtle">—</span>;

  return (
    <span className="flex items-center gap-1">
      <span className="max-w-40 truncate font-mono text-xs text-fg-muted">
        {masked && !shown ? "••••••••" : value}
      </span>

      {masked && (
        <button
          type="button"
          onClick={() => setShown((current) => !current)}
          aria-label={shown ? t("auth.hidePassword") : t("auth.showPassword")}
          title={shown ? t("auth.hidePassword") : t("auth.showPassword")}
          className="grid size-6 place-items-center rounded text-fg-subtle transition-colors hover:text-fg"
        >
          <Icon as={shown ? IconEyeOff : IconEye} size={13} />
        </button>
      )}

      <button
        type="button"
        onClick={() => {
          void navigator.clipboard.writeText(value);
          toast.success(t("cell.copied"));
        }}
        aria-label={t("cell.copy")}
        title={t("cell.copy")}
        className="grid size-6 place-items-center rounded text-fg-subtle transition-colors hover:text-fg"
      >
        <Icon as={IconCopy} size={13} />
      </button>
    </span>
  );
}

/**
 * Заведение и правка ключа.
 *
 * Платформа выбирается только при создании: `UPDATE` пишет ровно четыре
 * колонки — имя, роль, тип клиента и состояние (`api_keys.go:336`), —
 * и список платформ в форме правки был бы переключателем, который
 * ничего не переключает.
 *
 * Ограничений «запросов в секунду» и «в месяц» здесь нет: колонки
 * существуют, но не пишет их ни создание, ни правка (см. backend-notes).
 * Поле ввода, которое молча ничего не сохраняет, — это то, за что
 * переписан старый конструктор.
 */
function ApiKeyDialog({
  apiKey,
  roles,
  clientTypes,
  onClose,
}: {
  apiKey: ApiKey | null;
  roles: { id: string; name: string; clientTypeId: string }[];
  clientTypes: { id: string; name: string }[];
  onClose: () => void;
}) {
  const { t } = useTranslation();
  const create = useCreateApiKey();
  const update = useUpdateApiKey();
  const { platforms } = useClientPlatforms(!apiKey);

  const [draft, setDraft] = useState<ApiKeyDraft>({
    name: apiKey?.name ?? "",
    roleId: apiKey?.roleId ?? "",
    clientTypeId: apiKey?.clientTypeId ?? clientTypes[0]?.id ?? "",
    platformId: "",
    active: apiKey?.active ?? true,
  });

  /** Что показать после создания: секрет виден ровно один раз. */
  const [issued, setIssued] = useState<{ appId: string; appSecret: string } | null>(null);

  const put = (patch: Partial<ApiKeyDraft>) => setDraft((current) => ({ ...current, ...patch }));
  const busy = create.isPending || update.isPending;
  const platformId = draft.platformId || platforms[0]?.id || "";

  const typeRoles = roles.filter(
    (role) => !role.clientTypeId || role.clientTypeId === draft.clientTypeId,
  );

  if (issued) {
    return (
      <Modal onClose={onClose}>
        <div className="flex w-full max-w-lg flex-col gap-4 rounded-xl border border-border bg-surface p-5 shadow-modal">
          <div>
            <h2 className="text-base font-semibold">{t("apiKeys.issuedTitle")}</h2>
            <p className="mt-1 text-xs text-fg-subtle">{t("apiKeys.issuedHint")}</p>
          </div>

          <Field label={t("apiKeys.appId")}>
            <Input readOnly value={issued.appId} onFocus={(event) => event.target.select()} />
          </Field>

          <Field label={t("apiKeys.appSecret")}>
            <Input readOnly value={issued.appSecret} onFocus={(event) => event.target.select()} />
          </Field>

          <div className="flex justify-end">
            <Button onClick={onClose}>{t("action.close")}</Button>
          </div>
        </div>
      </Modal>
    );
  }

  return (
    <Modal onClose={onClose}>
      <form
        className="flex w-full max-w-md flex-col gap-4 rounded-xl border border-border bg-surface p-5 shadow-modal"
        onSubmit={(event) => {
          event.preventDefault();

          if (apiKey) {
            update.mutate({ id: apiKey.id, draft }, { onSuccess: onClose });
            return;
          }

          create.mutate(
            { ...draft, platformId },
            {
              onSuccess: (data) =>
                setIssued({ appId: data.app_id ?? "", appSecret: data.app_secret ?? "" }),
            },
          );
        }}
      >
        <h2 className="text-base font-semibold">
          {apiKey ? t("apiKeys.editTitle") : t("apiKeys.create")}
        </h2>

        <Field label={t("apiKeys.name")}>
          <Input
            autoFocus
            required
            value={draft.name}
            onChange={(event) => put({ name: event.target.value })}
          />
        </Field>

        <Field label={t("settings.clientType")}>
          <Select
            required
            value={draft.clientTypeId}
            onChange={(event) => put({ clientTypeId: event.target.value, roleId: "" })}
          >
            <option value="">—</option>
            {clientTypes.map((type) => (
              <option key={type.id} value={type.id}>
                {type.name}
              </option>
            ))}
          </Select>
        </Field>

        <Field label={t("settings.role")} hint={t("apiKeys.roleHint")}>
          <Select
            required
            value={draft.roleId}
            onChange={(event) => put({ roleId: event.target.value })}
          >
            <option value="">—</option>
            {typeRoles.map((role) => (
              <option key={role.id} value={role.id}>
                {role.name}
              </option>
            ))}
          </Select>
        </Field>

        {!apiKey && (
          <Field label={t("apiKeys.platform")} hint={t("apiKeys.platformHint")}>
            <Select value={platformId} onChange={(event) => put({ platformId: event.target.value })}>
              {platforms.map((platform) => (
                <option key={platform.id} value={platform.id}>
                  {platform.name}
                </option>
              ))}
            </Select>
          </Field>
        )}

        <label className="flex cursor-pointer items-center gap-2 text-sm">
          <Checkbox
            checked={draft.active}
            onChange={(event) => put({ active: event.target.checked })}
          />
          {t("apiKeys.active")}
        </label>

        <div className="flex justify-end gap-2">
          <Button type="button" variant="ghost" onClick={onClose}>
            {t("action.cancel")}
          </Button>
          <Button
            type="submit"
            disabled={busy || !draft.name.trim() || !draft.roleId || !draft.clientTypeId}
          >
            {busy ? t("common.saving") : apiKey ? t("action.save") : t("action.create")}
          </Button>
        </div>
      </form>
    </Modal>
  );
}

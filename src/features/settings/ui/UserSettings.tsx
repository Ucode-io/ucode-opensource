import { useDeferredValue, useState } from "react";
import { IconCopy, IconLink, IconPencil, IconPlus, IconTrash } from "@tabler/icons-react";
import { useTranslation } from "react-i18next";
import { useSession } from "@/shared/api/use-session";
import { toast } from "@/shared/lib/toast";
import { Button } from "@/shared/ui/button";
import { ConfirmDialog } from "@/shared/ui/confirm-dialog";
import { Icon } from "@/shared/ui/icon";
import { Dropdown } from "@/shared/ui/dropdown";
import { Field, Input } from "@/shared/ui/input";
import { Modal } from "@/shared/ui/modal";
import { PasswordInput } from "@/shared/ui/password-input";
import { Tabs } from "@/shared/ui/tabs";
import { useClientTypes } from "../api/client-types";
import { useRoles, type Role } from "../api/roles";
import {
  USERS_PAGE,
  useCreateUser,
  useDeleteUser,
  useUpdateUser,
  useUsers,
  type ProjectUser,
  type UserDraft,
} from "../api/users";
import { inviteLink } from "../model/invite";
import { Empty, Pager, SectionHeader, Td, Th } from "./parts";

/**
 * Люди проекта.
 *
 * Вкладки — типы клиентов, и это не выбор оформления: список приходит
 * по ОДНОМУ типу, общего «все люди проекта» в ручках нет (см. api/users).
 * У каждого типа своя таблица входа, свои роли и свои люди.
 *
 * Два способа завести человека, и оба настоящие:
 *
 * - **ссылкой** — человек приходит сам и назначает себе пароль. В ссылке
 *   лежит, куда его записать: проект, окружение, роль, тип клиента;
 * - **вручную** — мы придумываем и логин, и пароль. Так заводят
 *   служебные учётные записи, которым письмо слать некуда.
 *
 * Роль в форме отбирается по типу клиента вкладки: роль другого типа
 * бэкенд примет, а войти под ней человек не сможет.
 */
export function UserSettings() {
  const { t } = useTranslation();
  const { clientTypes, isLoading: loadingTypes } = useClientTypes();

  const [typeId, setTypeId] = useState("");
  const clientTypeId = typeId || clientTypes[0]?.id || "";

  const [query, setQuery] = useState("");
  /* Запрос уходит на сервер — печатать быстрее, чем он отвечает. */
  const search = useDeferredValue(query);
  const [page, setPage] = useState(1);

  const { users, count, isLoading } = useUsers({ clientTypeId, search, page });
  const { roles } = useRoles();
  const remove = useDeleteUser(clientTypeId);

  /** `null` — форма закрыта, `"new"` — новый человек, иначе правка. */
  const [editing, setEditing] = useState<ProjectUser | "new" | null>(null);
  const [inviting, setInviting] = useState(false);
  const [deleting, setDeleting] = useState<ProjectUser | null>(null);

  const typeRoles = rolesOf(roles, clientTypeId);
  const roleName = (id: string) => roles.find((role) => role.id === id)?.name ?? "";

  if (loadingTypes) return <p className="p-4 text-sm text-fg-muted">{t("common.loading")}</p>;
  if (!clientTypes.length) return <p className="p-4 text-sm text-fg-muted">{t("users.noTypes")}</p>;

  return (
    <div className="flex min-h-0 min-w-0 flex-1 flex-col">
      <SectionHeader title={t("users.title")} hint={t("users.hint")}>
        {/* Ширину задаёт обёртка: у `Input` в базовых классах стоит
            `w-full`, и своя ширина на самом поле с ним спорит. */}
        <div className="w-48">
          <Input
            value={query}
            onChange={(event) => {
              setQuery(event.target.value);
              setPage(1);
            }}
            placeholder={t("users.search")}
            aria-label={t("users.search")}
            className="h-7 text-sm"
          />
        </div>

        <Button size="sm" variant="secondary" onClick={() => setInviting(true)}>
          <Icon as={IconLink} size={14} />
          {t("users.invite")}
        </Button>

        <Button size="sm" onClick={() => setEditing("new")}>
          <Icon as={IconPlus} size={14} />
          {t("users.create")}
        </Button>
      </SectionHeader>

      {clientTypes.length > 1 && (
        <div className="shrink-0 px-4 py-2">
          <Tabs
            tabs={clientTypes.map((type) => ({ id: type.id, label: type.name }))}
            activeId={clientTypeId}
            onSelect={(id) => {
              setTypeId(id);
              setPage(1);
            }}
          />
        </div>
      )}

      <div className="min-h-0 flex-1 overflow-auto">
        <table className="w-full border-separate border-spacing-0">
          <thead>
            <tr>
              <Th>{t("settings.name")}</Th>
              <Th>{t("auth.login")}</Th>
              <Th>{t("auth.email")}</Th>
              <Th>{t("settings.phone")}</Th>
              <Th>{t("settings.role")}</Th>
              <Th className="w-20" />
            </tr>
          </thead>

          <tbody>
            {isLoading && <Empty text={t("common.loading")} colSpan={6} />}
            {!isLoading && !users.length && <Empty text={t("users.empty")} colSpan={6} />}

            {users.map((user) => (
              <tr key={user.id} className="group/row hover:bg-surface-hover">
                <Td>
                  <span className={user.active ? "" : "text-fg-subtle line-through"}>
                    {user.name || "—"}
                  </span>
                </Td>
                <Td className="font-mono text-xs">{user.login}</Td>
                <Td className="text-fg-muted">{user.email}</Td>
                <Td className="text-fg-muted">{user.phone}</Td>
                <Td className="text-fg-muted">{roleName(user.roleId)}</Td>

                <Td className="text-right">
                  <span className="inline-flex gap-1 opacity-0 transition-opacity group-hover/row:opacity-100 focus-within:opacity-100">
                    <IconButton
                      icon={IconPencil}
                      label={t("action.edit")}
                      onClick={() => setEditing(user)}
                    />
                    <IconButton
                      icon={IconTrash}
                      label={t("action.delete")}
                      danger
                      onClick={() => setDeleting(user)}
                    />
                  </span>
                </Td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <Pager page={page} total={count} limit={USERS_PAGE} onPage={setPage} />

      {editing && (
        <UserDialog
          user={editing === "new" ? null : editing}
          roles={typeRoles}
          clientTypeId={clientTypeId}
          onClose={() => setEditing(null)}
        />
      )}

      {inviting && (
        <InviteDialog
          roles={typeRoles}
          clientTypeId={clientTypeId}
          onClose={() => setInviting(false)}
        />
      )}

      {deleting && (
        <ConfirmDialog
          title={t("users.deleteTitle", { name: deleting.name || deleting.login })}
          description={t("users.deleteDescription")}
          confirmLabel={t("action.delete")}
          busy={remove.isPending}
          onClose={() => setDeleting(null)}
          onConfirm={() =>
            remove.mutate(deleting.id, { onSuccess: () => setDeleting(null) })
          }
        />
      )}
    </div>
  );
}

/**
 * Роли вкладки. Роль без типа клиента показываем всем: такие заведены
 * мимо нашей формы, и прятать их значит не дать выбрать единственную
 * существующую роль.
 */
function rolesOf(roles: Role[], clientTypeId: string): Role[] {
  return roles.filter((role) => !role.clientTypeId || role.clientTypeId === clientTypeId);
}

function IconButton({
  icon,
  label,
  danger = false,
  onClick,
}: {
  icon: typeof IconPencil;
  label: string;
  danger?: boolean;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-label={label}
      title={label}
      className={`grid size-7 place-items-center rounded-md text-fg-subtle transition-colors ${
        danger ? "hover:bg-danger-subtle hover:text-danger" : "hover:bg-surface-active hover:text-fg"
      }`}
    >
      <Icon as={icon} size={14} />
    </button>
  );
}

/**
 * Заведение и правка человека одной формой: поля те же, разница
 * в пароле. При правке его нет вовсе — менять чужой пароль отдельная
 * ручка и отдельный разговор, а поле «новый пароль» в форме правки
 * выглядит как «сотрётся, если не заполнить».
 */
function UserDialog({
  user,
  roles,
  clientTypeId,
  onClose,
}: {
  user: ProjectUser | null;
  roles: Role[];
  clientTypeId: string;
  onClose: () => void;
}) {
  const { t } = useTranslation();
  const create = useCreateUser(clientTypeId);
  const update = useUpdateUser();

  const [draft, setDraft] = useState<UserDraft>({
    name: user?.name ?? "",
    login: user?.login ?? "",
    email: user?.email ?? "",
    phone: user?.phone ?? "",
    password: "",
    roleId: user?.roleId ?? roles[0]?.id ?? "",
  });

  const put = (patch: Partial<UserDraft>) => setDraft((current) => ({ ...current, ...patch }));
  const busy = create.isPending || update.isPending;
  const filled = draft.login.trim() && draft.roleId && (user || draft.password);

  return (
    <Modal onClose={onClose}>
      <form
        className="flex w-full max-w-md flex-col gap-4 rounded-xl border border-border bg-surface p-5 shadow-modal"
        onSubmit={(event) => {
          event.preventDefault();
          if (user) update.mutate({ user, draft }, { onSuccess: onClose });
          else create.mutate(draft, { onSuccess: onClose });
        }}
      >
        <h2 className="text-base font-semibold">
          {user ? t("users.editTitle") : t("users.create")}
        </h2>

        <div className="grid grid-cols-2 gap-3">
          <Field label={t("settings.name")}>
            <Input
              autoFocus
              value={draft.name}
              onChange={(event) => put({ name: event.target.value })}
            />
          </Field>

          <Field label={t("auth.login")}>
            <Input
              required
              value={draft.login}
              onChange={(event) => put({ login: event.target.value })}
            />
          </Field>

          <Field label={t("auth.email")}>
            <Input
              type="email"
              value={draft.email}
              onChange={(event) => put({ email: event.target.value })}
            />
          </Field>

          <Field label={t("settings.phone")}>
            <Input value={draft.phone} onChange={(event) => put({ phone: event.target.value })} />
          </Field>

          {!user && (
            <Field label={t("auth.password")} hint={t("users.passwordHint")}>
              <PasswordInput
                required
                autoComplete="new-password"
                value={draft.password}
                onChange={(event) => put({ password: event.target.value })}
              />
            </Field>
          )}

          <Field label={t("settings.role")}>
            <Dropdown
              value={draft.roleId}
              placeholder="—"
              items={roles.map((role) => ({ value: role.id, label: role.name }))}
              onChange={(roleId) => put({ roleId })}
            />
          </Field>
        </div>

        <div className="flex justify-end gap-2">
          <Button type="button" variant="ghost" onClick={onClose}>
            {t("action.cancel")}
          </Button>
          <Button type="submit" disabled={!filled || busy}>
            {busy ? t("common.saving") : user ? t("action.save") : t("action.create")}
          </Button>
        </div>
      </form>
    </Modal>
  );
}

/**
 * Приглашение — это ссылка, а не письмо.
 *
 * Отправлять почту нам нечем: ручки приглашения, которая шлёт письмо,
 * в auth-сервисе нет — `/v2/user/invite` добавляет в проект УЖЕ
 * существующего человека по его идентификатору. Поэтому здесь то же,
 * что и в старой админке: адрес, который копируют и отправляют сами.
 *
 * Ссылка привязана к окружению, в котором мы сейчас: пришедший по ней
 * заводится именно в нём.
 */
function InviteDialog({
  roles,
  clientTypeId,
  onClose,
}: {
  roles: Role[];
  clientTypeId: string;
  onClose: () => void;
}) {
  const { t } = useTranslation();
  const store = useSession();
  const [roleId, setRoleId] = useState(roles[0]?.id ?? "");

  const link = inviteLink({
    origin: window.location.origin,
    projectId: store.getProjectId() ?? "",
    environmentId: store.getEnvironmentId() ?? "",
    roleId,
    clientTypeId,
  });

  return (
    <Modal onClose={onClose}>
      <div className="flex w-full max-w-lg flex-col gap-4 rounded-xl border border-border bg-surface p-5 shadow-modal">
        <div>
          <h2 className="text-base font-semibold">{t("users.invite")}</h2>
          <p className="mt-1 text-xs text-fg-subtle">{t("users.inviteHint")}</p>
        </div>

        <Field label={t("settings.role")}>
          <Dropdown
            value={roleId}
            placeholder="—"
            items={roles.map((role) => ({ value: role.id, label: role.name }))}
            onChange={setRoleId}
          />
        </Field>

        <Field label={t("users.inviteLink")}>
          <div className="flex gap-2">
            <Input readOnly value={link} onFocus={(event) => event.target.select()} />
            <Button
              type="button"
              variant="secondary"
              disabled={!link}
              onClick={() => {
                void navigator.clipboard.writeText(link);
                toast.success(t("cell.copied"));
              }}
            >
              <Icon as={IconCopy} size={14} />
              {t("cell.copy")}
            </Button>
          </div>
        </Field>

        <div className="flex justify-end">
          <Button variant="ghost" onClick={onClose}>
            {t("action.close")}
          </Button>
        </div>
      </div>
    </Modal>
  );
}

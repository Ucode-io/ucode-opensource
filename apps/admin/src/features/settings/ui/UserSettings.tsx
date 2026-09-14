import { useDeferredValue, useState } from "react";
import { IconCopy, IconPencil, IconPlus, IconTrash, IconX } from "@tabler/icons-react";
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
import { useClientTypes, type ClientType } from "../api/client-types";
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
 * «Новый пользователь» — один диалог (CreateUserDialog), а в нём четыре
 * настоящих способа завести человека, не украшение:
 *
 * - **Login** / **Phone** — вручную: логин и пароль придумываем сами.
 *   Так заводят служебные учётные записи, которым письмо слать некуда;
 * - **Email** — приглашение письмом. Логин и пароль генерируются на лету
 *   и никому не показываются: `POST /v2/user` сам шлёт их на почту, если
 *   поле `email` непустое (`user_service_v2.go:747`) — отдельной ручки
 *   приглашения у бэкенда нет, это побочный эффект создания;
 * - **Invite Link** — ссылка вместо письма: человек приходит сам и
 *   назначает себе пароль. Бэкенда под это тоже нет — `inviteLink()`
 *   просто зашивает в адрес проект, окружение, роль и тип клиента.
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
  const [limit, setLimit] = useState(USERS_PAGE);

  const { users, count, isLoading } = useUsers({ clientTypeId, search, page, limit });
  const { roles } = useRoles();
  const remove = useDeleteUser(clientTypeId);

  const [editing, setEditing] = useState<ProjectUser | null>(null);
  const [creating, setCreating] = useState(false);
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

        <Button size="sm" onClick={() => setCreating(true)}>
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

      <Pager
        page={page}
        total={count}
        limit={limit}
        onPage={setPage}
        onLimit={(next) => {
          setLimit(next);
          setPage(1);
        }}
      />

      {editing && (
        <UserDialog user={editing} roles={typeRoles} onClose={() => setEditing(null)} />
      )}

      {creating && (
        <CreateUserDialog
          clientTypes={clientTypes}
          roles={roles}
          clientTypeId={clientTypeId}
          onClose={() => setCreating(false)}
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
 * Правка человека. Пароль сюда не входит вовсе — менять чужой пароль
 * отдельная ручка и отдельный разговор, а поле «новый пароль» в форме
 * правки выглядит как «сотрётся, если не заполнить». Заведение нового
 * человека — отдельный диалог, `CreateUserDialog`, ниже.
 */
function UserDialog({
  user,
  roles,
  onClose,
}: {
  user: ProjectUser;
  roles: Role[];
  onClose: () => void;
}) {
  const { t } = useTranslation();
  const update = useUpdateUser();

  const [draft, setDraft] = useState<UserDraft>({
    name: user.name,
    login: user.login,
    email: user.email,
    phone: user.phone,
    password: "",
    roleId: user.roleId,
  });

  const put = (patch: Partial<UserDraft>) => setDraft((current) => ({ ...current, ...patch }));
  const filled = draft.login.trim() && draft.roleId;

  return (
    <Modal onClose={onClose}>
      <form
        className="flex w-full max-w-md flex-col gap-4 rounded-xl border border-border bg-surface p-5 shadow-modal"
        onSubmit={(event) => {
          event.preventDefault();
          update.mutate({ user, draft }, { onSuccess: onClose });
        }}
      >
        <h2 className="text-base font-semibold">{t("users.editTitle")}</h2>

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

        <Field label={t("settings.role")}>
          <Dropdown
            value={draft.roleId}
            placeholder="—"
            items={roles.map((role) => ({ value: role.id, label: role.name }))}
            onChange={(roleId) => put({ roleId })}
          />
        </Field>

        <div className="flex justify-end gap-2">
          <Button type="button" variant="ghost" onClick={onClose}>
            {t("action.cancel")}
          </Button>
          <Button type="submit" disabled={!filled || update.isPending}>
            {update.isPending ? t("common.saving") : t("action.save")}
          </Button>
        </div>
      </form>
    </Modal>
  );
}

type CreateTab = "login" | "phone" | "email" | "invite";

const CREATE_TABS: CreateTab[] = ["login", "phone", "email", "invite"];

/**
 * Пароль, который никто не вводит и не запоминает: письмо на вкладке
 * Email принесёт его само (см. комментарий у `CreateUserDialog`).
 */
function randomInvitePassword() {
  return crypto.randomUUID().replace(/-/g, "");
}

/**
 * Заведение нового человека — четыре вкладки, и на каждой свой способ,
 * не просто раскладка одной формы по частям:
 *
 * - **Login** / **Phone** — вручную, с паролем, который придумываем сами;
 * - **Email** — логин и пароль генерируются вслепую (`randomInvitePassword`)
 *   и уходят письмом: `POST /v2/user` сам шлёт их, если `email` непустой
 *   (`user_service_v2.go:747`). Тело письма бэкенд собирает из полей
 *   `login`/`password` запроса как есть, поэтому логином здесь ставим
 *   саму почту — иначе в письме придёт пустая строка;
 * - **Invite Link** — без обращения к бэкенду вовсе, см. `inviteLink()`.
 *
 * Переключение вкладки чистит поля идентификации: если оставить в
 * состоянии email с прошлой вкладки, при сабмите с Login backend увидит
 * непустой `email` и молча отправит письмо, которого никто не просил.
 *
 * Тип клиента тоже выбирается прямо здесь, а не только вкладкой списка
 * за модалкой: диалог открывают и с намерением завести человека в ДРУГОМ
 * типе, не переключая вид позади. Роль зависит от типа клиента (`rolesOf`)
 * — смена типа сбрасывает роль на первую подходящую, иначе в форме могла
 * бы остаться роль чужого типа, которую бэкенд примет, а войти под ней
 * не выйдет (см. комментарий у `UserSettings`).
 */
function CreateUserDialog({
  clientTypes,
  roles,
  clientTypeId: initialClientTypeId,
  onClose,
}: {
  clientTypes: ClientType[];
  roles: Role[];
  clientTypeId: string;
  onClose: () => void;
}) {
  const { t } = useTranslation();
  const store = useSession();

  const [clientTypeId, setClientTypeId] = useState(initialClientTypeId);
  const typeRoles = rolesOf(roles, clientTypeId);
  const create = useCreateUser(clientTypeId);

  const [tab, setTab] = useState<CreateTab>("login");
  const [draft, setDraft] = useState<UserDraft>({
    name: "",
    login: "",
    email: "",
    phone: "",
    password: "",
    roleId: typeRoles[0]?.id ?? "",
  });

  const put = (patch: Partial<UserDraft>) => setDraft((current) => ({ ...current, ...patch }));

  const selectClientType = (next: string) => {
    setClientTypeId(next);
    put({ roleId: rolesOf(roles, next)[0]?.id ?? "" });
  };

  const selectTab = (next: CreateTab) => {
    setTab(next);
    setDraft((current) => ({ ...current, login: "", phone: "", email: "", password: "" }));
  };

  const filled =
    tab === "email"
      ? Boolean(draft.email.trim() && draft.roleId)
      : Boolean(draft.login.trim() && draft.password.trim() && draft.roleId);

  const link = inviteLink({
    origin: window.location.origin,
    projectId: store.getProjectId() ?? "",
    environmentId: store.getEnvironmentId() ?? "",
    roleId: draft.roleId,
    clientTypeId,
  });

  const tabs = CREATE_TABS.map((id) => ({
    id,
    label:
      id === "login"
        ? t("auth.login")
        : id === "phone"
          ? t("settings.phone")
          : id === "email"
            ? t("auth.email")
            : t("users.inviteLink"),
  }));

  return (
    <Modal onClose={onClose}>
      <form
        className="flex w-full max-w-2xl flex-col gap-3 rounded-xl border border-border bg-surface p-5 shadow-modal"
        onSubmit={(event) => {
          event.preventDefault();
          if (tab === "invite") return;
          const payload =
            tab === "email"
              ? { ...draft, login: draft.email.trim(), password: randomInvitePassword() }
              : draft;
          create.mutate(payload, { onSuccess: onClose });
        }}
      >
        <div className="flex items-center justify-between">
          <h2 className="text-base font-semibold">{t("users.create")}</h2>
          <button
            type="button"
            onClick={onClose}
            aria-label={t("action.close")}
            title={t("action.close")}
            className="grid size-7 shrink-0 place-items-center rounded-md text-fg-muted transition-colors hover:bg-surface-hover hover:text-fg"
          >
            <Icon as={IconX} size={16} />
          </button>
        </div>

        {/* Тип клиента, роль и способ — в одну строку: все три нужны
            при любом табе, и это не четыре формы, а одна форма с четырьмя
            переключаемыми полями идентификации. */}
        <div className="flex items-center gap-2">
          <Dropdown
            size="control"
            className="w-40 shrink-0"
            value={clientTypeId}
            placeholder={t("settings.clientType")}
            items={clientTypes.map((type) => ({ value: type.id, label: type.name }))}
            onChange={selectClientType}
          />
          <Dropdown
            size="control"
            className="w-40 shrink-0"
            value={draft.roleId}
            placeholder={t("settings.role")}
            items={typeRoles.map((role) => ({ value: role.id, label: role.name }))}
            onChange={(roleId) => put({ roleId })}
          />
          <Tabs tabs={tabs} activeId={tab} onSelect={(id) => selectTab(id as CreateTab)} />
        </div>

        {/*
          Высота зафиксирована на самой длинной вкладке (Phone: 4 поля по
          36px + 3 зазора по 12px = 180px), чтобы модалка не прыгала —
          `Modal` центрирует её через `place-items-center`, и без этого
          каждая смена таба меняла высоту формы и вместе с ней сдвигала
          окно по экрану.
        */}
        <div className="flex min-h-[180px] flex-col gap-3">
          {tab === "invite" ? (
            <div className="flex gap-2">
              <Input readOnly value={link} onFocus={(event) => event.target.select()} />
              <Button
                type="button"
                variant="secondary"
                disabled={!link}
                /* Кнопка у Button — 32px (--spacing-control), а у Input рядом
                   36px (--spacing-input): классы одной и той же утилиты
                   конфликтуют непредсказуемо (см. комментарий в dropdown.tsx),
                   поэтому высоту здесь выравнивает инлайн-стиль, а не className. */
                style={{ height: "var(--spacing-input)" }}
                onClick={() => {
                  void navigator.clipboard.writeText(link);
                  toast.success(t("cell.copied"));
                }}
              >
                <Icon as={IconCopy} size={14} />
                {t("cell.copy")}
              </Button>
            </div>
          ) : (
            <>
              <Input
                autoFocus
                placeholder={t("settings.name")}
                value={draft.name}
                onChange={(event) => put({ name: event.target.value })}
              />

              {tab === "email" ? (
                <>
                  <Input
                    required
                    type="email"
                    placeholder={t("auth.email")}
                    value={draft.email}
                    onChange={(event) => put({ email: event.target.value })}
                  />
                  <p className="text-xs text-fg-subtle">{t("users.emailInviteHint")}</p>
                </>
              ) : (
                <>
                  <Input
                    required
                    placeholder={t("auth.login")}
                    value={draft.login}
                    onChange={(event) => put({ login: event.target.value })}
                  />

                  {tab === "phone" && (
                    <Input
                      placeholder={t("settings.phone")}
                      value={draft.phone}
                      onChange={(event) => put({ phone: event.target.value })}
                    />
                  )}

                  <PasswordInput
                    required
                    autoComplete="new-password"
                    placeholder={t("auth.password")}
                    value={draft.password}
                    onChange={(event) => put({ password: event.target.value })}
                  />
                </>
              )}
            </>
          )}
        </div>

        {/* Высота держится и на пустом табе: иначе исчезновение кнопки
            на вкладке «Ссылка» тоже двигало бы окно. */}
        <div className="flex h-8 justify-end">
          {tab !== "invite" && (
            <Button type="submit" disabled={!filled || create.isPending}>
              {create.isPending ? t("common.saving") : t("action.create")}
            </Button>
          )}
        </div>
      </form>
    </Modal>
  );
}

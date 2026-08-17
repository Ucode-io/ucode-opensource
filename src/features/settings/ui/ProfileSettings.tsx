import { useEffect, useState } from "react";
import { IconDeviceDesktop, IconTrash } from "@tabler/icons-react";
import { useTranslation } from "react-i18next";
import { useSession } from "@/shared/api/use-session";
import { Button } from "@/shared/ui/button";
import { Icon } from "@/shared/ui/icon";
import { Field, Input } from "@/shared/ui/input";
import { PasswordInput } from "@/shared/ui/password-input";
import {
  useChangePassword,
  useDeleteSession,
  useProfile,
  useSessions,
  useUpdateProfile,
  type ProfileDraft,
} from "../api/profile";

/**
 * Профиль: имя, как человека зовут в интерфейсе, и способы входа.
 *
 * Роль и тип клиента показаны, но не правятся: их выдаёт проект, а не
 * человек себе сам. Поле ввода, которое сервер всё равно перезапишет,
 * — это ложное обещание.
 */
export function ProfileSettings() {
  const { t } = useTranslation();
  const { profile, isLoading } = useProfile();
  const update = useUpdateProfile();
  const store = useSession();
  const role = store.getProfile()?.role ?? "";

  const [draft, setDraft] = useState<ProfileDraft>({
    name: "",
    login: "",
    email: "",
    phone: "",
  });

  // Черновик наполняется, когда профиль приехал: до этого править нечего.
  useEffect(() => {
    if (!profile) return;
    setDraft({
      name: profile.name,
      login: profile.login,
      email: profile.email,
      phone: profile.phone,
    });
  }, [profile]);

  if (isLoading || !profile) {
    return <p className="text-sm text-fg-subtle">{t("common.loading")}</p>;
  }

  const changed =
    draft.name !== profile.name ||
    draft.login !== profile.login ||
    draft.email !== profile.email ||
    draft.phone !== profile.phone;

  return (
    <div className="flex max-w-2xl flex-col gap-6">
      <section className="flex flex-col gap-4">
        <div className="grid grid-cols-2 gap-4">
          <Field label={t("settings.name")}>
            <Input
              value={draft.name}
              onChange={(event) => setDraft({ ...draft, name: event.target.value })}
            />
          </Field>

          <Field label={t("auth.login")}>
            <Input
              value={draft.login}
              onChange={(event) => setDraft({ ...draft, login: event.target.value })}
            />
          </Field>

          <Field label={t("auth.email")}>
            <Input
              type="email"
              value={draft.email}
              onChange={(event) => setDraft({ ...draft, email: event.target.value })}
            />
          </Field>

          <Field label={t("settings.phone")}>
            <Input
              value={draft.phone}
              onChange={(event) => setDraft({ ...draft, phone: event.target.value })}
            />
          </Field>

          {/* Роль выдаёт проект, а не человек себе сам. */}
          <Field label={t("settings.role")}>
            <Input value={role} disabled readOnly />
          </Field>
        </div>

        <div className="flex justify-end">
          <Button
            disabled={!changed || update.isPending}
            onClick={() => update.mutate({ profile, draft })}
          >
            {t("action.save")}
          </Button>
        </div>
      </section>

      <PasswordSection />
      <SessionsSection />
    </div>
  );
}

/**
 * Смена пароля. Отдельной формой: для неё нужен прежний пароль, и
 * ошибка в нём значит «это не вы», а не «не сохранилось».
 */
function PasswordSection() {
  const { t } = useTranslation();
  const change = useChangePassword();
  const [oldPassword, setOldPassword] = useState("");
  const [password, setPassword] = useState("");
  const [repeat, setRepeat] = useState("");

  const filled = Boolean(oldPassword && password);
  const matches = password === repeat;

  return (
    <section className="flex flex-col gap-4 border-t border-border pt-6">
      <h3 className="text-sm font-medium">{t("settings.password")}</h3>

      <div className="grid grid-cols-3 gap-4">
        <Field label={t("settings.oldPassword")}>
          {/* Браузеру здесь подставлять нечего: это не форма входа,
              и подставленный им пароль человек примет за уже введённый. */}
          <PasswordInput
            autoComplete="new-password"
            value={oldPassword}
            onChange={(event) => setOldPassword(event.target.value)}
          />
        </Field>

        <Field label={t("settings.newPassword")}>
          <PasswordInput
            autoComplete="new-password"
            value={password}
            onChange={(event) => setPassword(event.target.value)}
          />
        </Field>

        <Field
          label={t("settings.repeatPassword")}
          {...(repeat && !matches ? { hint: t("settings.passwordMismatch") } : {})}
        >
          <PasswordInput
            autoComplete="new-password"
            value={repeat}
            onChange={(event) => setRepeat(event.target.value)}
          />
        </Field>
      </div>

      <div className="flex justify-end">
        <Button
          variant="secondary"
          disabled={!filled || !matches || change.isPending}
          onClick={() =>
            change.mutate(
              { oldPassword, password },
              {
                onSuccess: () => {
                  setOldPassword("");
                  setPassword("");
                  setRepeat("");
                },
              },
            )
          }
        >
          {t("settings.changePassword")}
        </Button>
      </div>
    </section>
  );
}

/**
 * Открытые сессии: где человек ещё залогинен.
 *
 * Текущая помечена и не закрывается кнопкой: закрыть её — это выйти,
 * и для выхода есть выход. Кнопка, которая молча разлогинивает из
 * настроек, читается как поломка.
 */
function SessionsSection() {
  const { t, i18n } = useTranslation();
  const { sessions, isLoading } = useSessions();
  const remove = useDeleteSession();

  return (
    <section className="flex flex-col gap-3 border-t border-border pt-6">
      <h3 className="text-sm font-medium">{t("settings.sessions")}</h3>

      {isLoading && <p className="text-sm text-fg-subtle">{t("common.loading")}</p>}
      {!isLoading && !sessions.length && (
        <p className="text-sm text-fg-subtle">{t("settings.noSessions")}</p>
      )}

      {sessions.map((item) => (
        <div
          key={item.id}
          className="flex items-center gap-3 rounded-md border border-border px-3 py-2"
        >
          <Icon as={IconDeviceDesktop} size={16} className="shrink-0 text-fg-muted" />

          <div className="min-w-0 flex-1">
            <p className="truncate text-sm text-fg">
              {item.device || t("settings.unknownDevice")}
              {item.current && (
                <span className="ml-2 rounded bg-accent-subtle px-1.5 py-0.5 text-2xs text-accent-text">
                  {t("settings.currentSession")}
                </span>
              )}
            </p>
            <p className="truncate text-2xs text-fg-subtle">
              {[item.ip, formatDate(item.updatedAt, i18n.language)].filter(Boolean).join(" · ")}
            </p>
          </div>

          {!item.current && (
            <button
              type="button"
              onClick={() => remove.mutate(item.id)}
              aria-label={t("settings.closeSession")}
              title={t("settings.closeSession")}
              className="grid size-7 shrink-0 place-items-center rounded-md text-fg-subtle transition-colors hover:bg-danger-subtle hover:text-danger"
            >
              <Icon as={IconTrash} size={14} />
            </button>
          )}
        </div>
      ))}
    </section>
  );
}

/** Дата сессии — как её отдал сервер. Мусор не показываем вовсе. */
function formatDate(value: string, locale: string): string {
  if (!value) return "";

  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? "" : date.toLocaleString(locale);
}

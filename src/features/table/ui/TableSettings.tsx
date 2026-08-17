import { useState } from "react";
import { IconLoader2 } from "@tabler/icons-react";
import { useTranslation } from "react-i18next";
import type { TranslationKey } from "@/shared/lib/i18n";
import { Checkbox } from "@/shared/ui/checkbox";
import { CommitInput } from "@/shared/ui/commit-input";
import { Icon } from "@/shared/ui/icon";
import {
  LOGIN_STRATEGIES,
  useTableSettings,
  useUpdateTableSettings,
  type LoginStrategy,
} from "../api/table-settings";

/**
 * Настройки ТАБЛИЦЫ: имя на языках данных, кэш, мягкое удаление, ручная
 * сортировка и вход. То, что в старой админке звалось «General».
 *
 * Правки уходят по одной и сразу — как остальные настройки в этой
 * панели. Кнопки «сохранить» нет: она нужна там, где правку можно
 * отменить, а здесь отменяют повторным щелчком.
 *
 * Чего здесь нет и почему:
 *
 *   Слаг — показан, но не правится: в UPDATE его колонки нет вовсе
 *   (object_builder, storage/postgres/table.go:887). Поле ввода
 *   в старой админке принимало новый слаг, отвечало «сохранено»
 *   и не меняло ничего.
 *
 *   Шесть выпадающих списков таблицы входа (тип пользователя, роль,
 *   логин, пароль, email, телефон) — бэкенд их перезаписывает своим:
 *   role_id и client_type_id он кладёт строками, а поля логина заводит
 *   сам по выбранным способам входа (table.go:1007). Осталось то, что
 *   действительно решает, — способы входа.
 */
export function TableSettings({
  tableSlug,
  /** Языки ДАННЫХ проекта: имя таблицы задаётся на каждом. */
  languages,
}: {
  tableSlug: string;
  languages: { code: string; nativeName: string }[];
}) {
  const { t } = useTranslation();
  const { table, isLoading } = useTableSettings(tableSlug);
  const update = useUpdateTableSettings(tableSlug);

  /*
   * Включённая таблица входа без единого способа на сервер не уезжает:
   * ручка отвечает отказом «login_strategy does not exist»
   * (table.go:970). Поэтому галка сначала живёт здесь и превращается
   * в запрос вместе с первым выбранным способом.
   */
  const [wantLogin, setWantLogin] = useState(false);

  if (isLoading || !table) {
    return <p className="px-2 py-1.5 text-xs text-fg-subtle">{t("common.loading")}</p>;
  }

  const login = table.isLoginTable || wantLogin;
  const strategies = table.loginStrategies;

  const toggleStrategy = (strategy: LoginStrategy) => {
    const next = strategies.includes(strategy)
      ? strategies.filter((item) => item !== strategy)
      : [...strategies, strategy];

    // Последний снятый способ выключает и саму таблицу входа: входить
    // в неё стало нечем, а отказ ручки об этом не скажет.
    if (!next.length) {
      setWantLogin(false);
      update.mutate({ table, isLoginTable: false, loginStrategies: [] });
      return;
    }

    update.mutate({ table, isLoginTable: true, loginStrategies: next });
  };

  return (
    /* Прокрутка своя: у таблицы входа страница длиннее экрана, и без
       неё нижние настройки уезжали за нижний край окна. */
    <div className="flex max-h-[70vh] flex-col gap-3 overflow-y-auto p-2">
      <div className="flex flex-col gap-1.5">
        {languages.map((language) => (
          <label key={language.code} className="flex flex-col gap-0.5">
            <span className="px-0.5 text-2xs text-fg-subtle">{language.nativeName}</span>
            <CommitInput
              value={table.labels[language.code] ?? ""}
              placeholder={table.label || table.slug}
              label={t("tableSettings.name")}
              onCommit={(name) =>
                update.mutate({
                  table,
                  labels: { ...table.labels, [language.code]: name },
                })
              }
            />
          </label>
        ))}
      </div>

      {/* Слаг — не поле ввода: его этой ручкой не сменить (см. выше).
          Показан потому, что он и есть имя таблицы в адресах и в API. */}
      <div className="flex flex-col gap-0.5">
        <span className="px-0.5 text-2xs text-fg-subtle">{t("tableSettings.slug")}</span>
        <p className="truncate rounded-md bg-surface-active px-2.5 py-1.5 font-mono text-xs text-fg-muted">
          {table.slug}
        </p>
      </div>

      <div className="flex flex-col gap-1">
        <Toggle
          label={t("tableSettings.cache")}
          hint={t("tableSettings.cacheHint")}
          checked={table.isCached}
          onChange={(isCached) => update.mutate({ table, isCached })}
        />
        <Toggle
          label={t("tableSettings.softDelete")}
          hint={t("tableSettings.softDeleteHint")}
          checked={table.softDelete}
          onChange={(softDelete) => update.mutate({ table, softDelete })}
        />
        <Toggle
          label={t("tableSettings.sort")}
          hint={t("tableSettings.sortHint")}
          checked={table.orderBy}
          onChange={(orderBy) => update.mutate({ table, orderBy })}
        />
        <Toggle
          label={t("tableSettings.loginTable")}
          hint={t("tableSettings.loginTableHint")}
          checked={login}
          onChange={(next) => {
            // Выключение уезжает сразу, включение ждёт способа входа.
            if (!next) {
              setWantLogin(false);
              if (table.isLoginTable) update.mutate({ table, isLoginTable: false });
              return;
            }

            setWantLogin(true);
          }}
        />
      </div>

      {login && (
        <div className="flex flex-col gap-1 rounded-md bg-surface-active/50 p-1.5">
          <p className="px-0.5 text-2xs text-fg-subtle">{t("tableSettings.strategiesHint")}</p>

          {LOGIN_STRATEGIES.map((strategy) => (
            <Toggle
              key={strategy}
              label={t(`tableSettings.strategy.${strategy}` as TranslationKey)}
              checked={strategies.includes(strategy)}
              onChange={() => toggleStrategy(strategy)}
            />
          ))}

          {table.isLoginTable && (
            <Toggle
              label={t("tableSettings.lastActivity")}
              hint={t("tableSettings.lastActivityHint")}
              checked={table.lastActivity}
              onChange={(lastActivity) => update.mutate({ table, lastActivity })}
            />
          )}
        </div>
      )}

      {update.isPending && (
        <p className="flex items-center gap-1 px-0.5 text-2xs text-fg-subtle">
          <Icon as={IconLoader2} size={12} className="animate-spin" />
          {t("common.saving")}
        </p>
      )}
    </div>
  );
}

/** Флажок со своей подписью и пояснением под ней. */
function Toggle({
  label,
  hint,
  checked,
  onChange,
}: {
  label: string;
  hint?: string;
  checked: boolean;
  onChange: (checked: boolean) => void;
}) {
  return (
    <label className="flex cursor-pointer items-start gap-2 rounded-md px-1 py-1 transition-colors hover:bg-surface-hover">
      <span className="pt-0.5">
        <Checkbox checked={checked} onChange={(event) => onChange(event.target.checked)} />
      </span>

      <span className="flex min-w-0 flex-col">
        <span className="text-sm text-fg">{label}</span>
        {hint && <span className="text-2xs text-fg-subtle">{hint}</span>}
      </span>
    </label>
  );
}

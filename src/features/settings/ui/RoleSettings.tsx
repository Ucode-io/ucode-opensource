import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import type { TranslationKey } from "@/shared/lib/i18n";
import { Button } from "@/shared/ui/button";
import { Checkbox } from "@/shared/ui/checkbox";
import { useRolePermissions, useRoles, useUpdateRolePermissions } from "../api/roles";
import {
  RECORD_RIGHTS,
  SCREEN_RIGHTS,
  toggleRight,
  type RolePermissions,
} from "../model/permissions";

/**
 * Роли и их права на таблицы.
 *
 * Роль слева, её таблицы справа — по строке на таблицу и по флажку
 * на право. Матрица, а не список экранов настройки у каждой таблицы:
 * права раздают, сравнивая роли между собой, и ради этого их нужно
 * видеть рядом.
 *
 * Прав в ответе больше, чем здесь показано. Показаны те, что у нас
 * действительно что-то решают, — по ним прячутся кнопки в таблице
 * (features/auth/model/permissions). Остальные уезжают обратно
 * нетронутыми: см. model/permissions.
 *
 * Правки копятся и уходят по кнопке, а не по флажку: PUT перезаписывает
 * права роли ЦЕЛИКОМ, и запрос на каждый щелчок означал бы гонку, где
 * побеждает последний ответ, а не последний щелчок.
 */
export function RoleSettings() {
  const { t } = useTranslation();
  const { roles, isLoading } = useRoles();

  const [roleId, setRoleId] = useState("");
  const active = roleId || roles[0]?.id || "";

  const { permissions, isLoading: loadingPermissions } = useRolePermissions(active);
  const update = useUpdateRolePermissions(active);

  /** Черновик: правки видно сразу, а уезжают они по кнопке. */
  const [draft, setDraft] = useState<RolePermissions | null>(null);
  useEffect(() => setDraft(permissions ?? null), [permissions]);

  const dirty = Boolean(draft && permissions && draft.raw !== permissions.raw);

  if (isLoading) return <p className="p-4 text-sm text-fg-muted">{t("common.loading")}</p>;
  if (!roles.length) return <p className="p-4 text-sm text-fg-muted">{t("roles.empty")}</p>;

  return (
    <div className="flex min-h-0 flex-1">
      {/* Роли — вторым столбцом слева: их единицы, и переключаются они
          чаще, чем что-либо ещё на этом экране. */}
      <nav className="flex w-44 shrink-0 flex-col gap-0.5 overflow-y-auto border-r border-border p-2">
        {roles.map((role) => (
          <button
            key={role.id}
            type="button"
            onClick={() => setRoleId(role.id)}
            className={`flex h-8 shrink-0 items-center rounded-md px-2 text-left text-sm transition-colors ${
              role.id === active
                ? "bg-surface-active text-fg"
                : "text-fg-muted hover:bg-surface-hover hover:text-fg"
            }`}
          >
            <span className="truncate">{role.name}</span>
          </button>
        ))}
      </nav>

      <div className="flex min-h-0 flex-1 flex-col">
        {loadingPermissions || !draft ? (
          <p className="p-4 text-sm text-fg-muted">{t("common.loading")}</p>
        ) : (
          <>
            <div className="min-h-0 flex-1 overflow-auto">
              <table className="w-full min-w-max border-separate border-spacing-0 text-sm">
                <thead className="sticky top-0 z-10 bg-surface">
                  <tr>
                    <th className="h-9 border-b border-border px-3 text-left font-normal text-fg-muted">
                      {t("roles.table")}
                    </th>

                    {[...RECORD_RIGHTS, ...SCREEN_RIGHTS].map((right, index) => (
                      <th
                        key={right}
                        /* Тонкая граница отделяет права на строки от прав
                           на экран: колонок одиннадцать, и без неё они
                           читаются одним рядом. */
                        className={`h-9 w-24 border-b border-border px-2 text-center text-xs font-normal text-fg-muted ${
                          index === RECORD_RIGHTS.length ? "border-l" : ""
                        }`}
                      >
                        {t(`roles.right.${right}` as TranslationKey)}
                      </th>
                    ))}
                  </tr>
                </thead>

                <tbody>
                  {draft.tables.map((table) => (
                    <tr key={table.slug} className="hover:bg-surface-hover">
                      <td className="h-9 truncate border-b border-border px-3">
                        <span className="text-fg">{table.label}</span>
                        <span className="ml-2 font-mono text-2xs text-fg-subtle">{table.slug}</span>
                      </td>

                      {RECORD_RIGHTS.map((right) => (
                        <td key={right} className="h-9 border-b border-border text-center">
                          <Checkbox
                            checked={table.record[right]}
                            aria-label={`${table.label}: ${t(`roles.right.${right}` as TranslationKey)}`}
                            onChange={(event) =>
                              setDraft(toggleRight(draft, table.slug, right, event.target.checked))
                            }
                          />
                        </td>
                      ))}

                      {SCREEN_RIGHTS.map((right, index) => (
                        <td
                          key={right}
                          className={`h-9 border-b border-border text-center ${index === 0 ? "border-l" : ""}`}
                        >
                          <Checkbox
                            checked={table.screen[right]}
                            aria-label={`${table.label}: ${t(`roles.right.${right}` as TranslationKey)}`}
                            onChange={(event) =>
                              setDraft(toggleRight(draft, table.slug, right, event.target.checked))
                            }
                          />
                        </td>
                      ))}
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>

            <div className="flex h-12 shrink-0 items-center justify-between gap-3 border-t border-border px-3">
              <p className="text-xs text-fg-subtle">{t("roles.hint")}</p>

              <Button
                size="sm"
                disabled={!dirty || update.isPending}
                onClick={() => update.mutate(draft)}
              >
                {update.isPending ? t("common.saving") : t("action.save")}
              </Button>
            </div>
          </>
        )}
      </div>
    </div>
  );
}

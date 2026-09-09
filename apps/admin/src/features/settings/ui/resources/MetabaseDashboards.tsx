import { IconCopy, IconLoader2 } from "@tabler/icons-react";
import { useTranslation } from "react-i18next";
import { Icon } from "@/shared/ui/icon";
import { toast } from "@/shared/lib/toast";
import { useMetabaseDashboards, useMetabasePublicUrl } from "../../api/metabase";

/**
 * Дашборды ресурса METABASE: список из самого Metabase и публичная
 * ссылка на каждый.
 *
 * Список приходит по логину и паролю, которые уже показаны формой выше,
 * — своего поля тут нет. Учётку выдаёт бэкенд при создании ресурса
 * (METABASE — readOnly), поэтому пока строка не сохранена, спрашивать
 * нечем и блок не показывается.
 */
export function MetabaseDashboards({ username, password }: { username: string; password: string }) {
  const { t } = useTranslation();
  const { dashboards, isLoading, error } = useMetabaseDashboards(username, password);
  const publicUrl = useMetabasePublicUrl();

  if (!username || !password) return null;

  return (
    <div className="flex flex-col gap-1">
      <span className="px-0.5 text-xs text-fg-muted">{t("resources.dashboards")}</span>

      {isLoading && <p className="px-1 text-xs text-fg-subtle">{t("common.loading")}</p>}

      {!isLoading && !dashboards.length && (
        <p className="px-1 text-xs text-fg-subtle">{error ?? t("resources.dashboardsEmpty")}</p>
      )}

      <div className="flex max-h-56 flex-col overflow-y-auto">
        {dashboards.map((dashboard) => (
          <div
            key={dashboard.id}
            className="flex h-8 items-center gap-2 rounded-md px-2 hover:bg-surface-hover"
          >
            <span className="min-w-0 flex-1 truncate text-sm text-fg">
              {dashboard.name || `#${dashboard.id}`}
            </span>

            {/*
             * «Скопировать», а не «открыть»: ссылку заводят, чтобы
             * вставить её в чужую страницу или пункт меню. Так же
             * делает и старая админка (`MetabaseContent.jsx:44`).
             */}
            <button
              type="button"
              disabled={publicUrl.isPending}
              onClick={() =>
                publicUrl.mutate(dashboard.id, {
                  onSuccess: (data) => {
                    // Пустой адрес при успешном ответе — тоже отказ,
                    // и молчать о нём нельзя: «нажал, и ничего»
                    // читается как поломка кнопки.
                    if (!data.url) {
                      toast.error(t("resources.metabaseFailed"));
                      return;
                    }

                    void navigator.clipboard.writeText(data.url);
                    toast.success(t("cell.copied"));
                  },
                })
              }
              aria-label={t("resources.copyPublicUrl")}
              title={t("resources.copyPublicUrl")}
              className="grid size-6 shrink-0 place-items-center rounded text-fg-subtle transition-colors hover:bg-surface-active hover:text-fg disabled:opacity-40"
            >
              <Icon
                as={publicUrl.isPending ? IconLoader2 : IconCopy}
                size={14}
                className={publicUrl.isPending ? "animate-spin" : ""}
              />
            </button>
          </div>
        ))}
      </div>

      {/* Публикация — не чтение: ссылка делает дашборд доступным всем,
          у кого она есть. Об этом лучше знать до нажатия. */}
      <p className="px-1 text-2xs text-fg-subtle">{t("resources.publicUrlHint")}</p>
    </div>
  );
}

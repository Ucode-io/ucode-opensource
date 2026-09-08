import { useEffect } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/shared/ui/button";
import {
  PROVIDERS,
  useConnectIntegration,
  useDisconnectIntegration,
  useIntegration,
  type ProviderId,
} from "../../api/integrations";

/**
 * Подключённые аккаунты репозиториев — GitHub, GitLab, Bitbucket.
 *
 * Отдельным блоком под списком ресурсов, а не полем внутри ресурса
 * GITHUB: это другая сущность и другая служба (см. api/integrations).
 * Токен сюда не приезжает вовсе — бэкенд обменивает код сам, наружу
 * отдаётся только имя пользователя.
 */
export function IntegrationAccounts() {
  const { t } = useTranslation();

  return (
    <div className="shrink-0 border-t border-border px-4 py-3">
      <p className="text-sm font-medium text-fg">{t("integrations.title")}</p>
      <p className="mt-0.5 mb-2 text-xs text-fg-subtle">{t("integrations.hint")}</p>

      <div className="flex flex-col gap-1">
        {PROVIDERS.map((provider) => (
          <ProviderRow key={provider.id} id={provider.id} label={provider.label} />
        ))}
      </div>
    </div>
  );
}

function ProviderRow({ id, label }: { id: ProviderId; label: string }) {
  const { t } = useTranslation();
  const { integration, isLoading, refetch } = useIntegration(id);
  const connect = useConnectIntegration();
  const disconnect = useDisconnectIntegration();

  /*
   * Авторизация уходит в соседнюю вкладку, и возвращается человек
   * сюда переключением окна — своего события у этого нет, кроме
   * фокуса. Поэтому перечитываем на возврат, а не опросом: опрос
   * ходил бы в сеть всё время, пока раздел открыт.
   */
  useEffect(() => {
    const onFocus = () => void refetch();
    window.addEventListener("focus", onFocus);
    return () => window.removeEventListener("focus", onFocus);
  }, [refetch]);

  return (
    <div className="flex h-8 items-center gap-2">
      <span className="w-24 shrink-0 text-sm text-fg">{label}</span>

      <span className="min-w-0 flex-1 truncate text-xs text-fg-muted">
        {isLoading
          ? t("common.loading")
          : integration
            ? integration.username || integration.name
            : t("integrations.notConnected")}
      </span>

      {integration ? (
        <Button
          size="sm"
          variant="ghost"
          disabled={disconnect.isPending}
          onClick={() => disconnect.mutate({ provider: id, id: integration.id })}
        >
          {t("integrations.disconnect")}
        </Button>
      ) : (
        <Button
          size="sm"
          variant="secondary"
          disabled={connect.isPending}
          onClick={() => connect.mutate(id)}
        >
          {t("integrations.connect")}
        </Button>
      )}
    </div>
  );
}

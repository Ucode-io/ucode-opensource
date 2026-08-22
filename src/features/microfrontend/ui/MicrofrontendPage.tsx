import { Component, Suspense, lazy, useMemo, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { useEnvironments } from "@/features/workspace";
import { errorText, http, httpAuth } from "@/shared/api/client";
import { useSession } from "@/shared/api/use-session";
import i18n from "@/shared/lib/i18n";
import { useMicrofrontend } from "../api/microfrontend";
import { loadRemotePage, type RemotePageProps } from "../model/remote";

/**
 * Экран пункта меню типа `MICROFRONTEND`: чужое приложение внутри админки.
 *
 * Сам пункт знает только идентификатор (`menu.microfrontend_id`); адрес
 * сборки дочитывается ручкой (см. api/microfrontend), а грузится модуль
 * через Module Federation (см. model/remote).
 */
export function MicrofrontendPage({
  id,
  params,
}: {
  id: string;
  /** `attributes.params` пункта меню — настройка запуска ремоута. */
  params: Record<string, string>;
}) {
  const { t } = useTranslation();
  const { microfrontend, isLoading, error } = useMicrofrontend(id);
  const environment = useEnvironmentKind();

  /*
   * Ремоут перемонтируется на каждый заход, а не оживает с прежним
   * состоянием: он держит своё дерево целиком, и показать чужой экран
   * с данными прошлого пункта хуже, чем загрузить его заново. Старая
   * админка делала то же самое ключом `activationKey`.
   */
  const activationKey = `${id}:${microfrontend?.url ?? ""}`;

  const Page = useMemo(() => {
    const url = microfrontend?.url;
    if (!url) return null;

    return lazy(async () => ({ default: await loadRemotePage(id, url) }));
  }, [id, microfrontend?.url]);

  // Пункт есть, приложение к нему не привязано: у бэкенда `microfrontend_id`
  // необязателен, а сменить его правкой пункта нельзя — только завести заново.
  if (!id) return <Message text={t("microfrontend.notPicked")} />;
  if (isLoading) return <Message text={t("common.loading")} />;
  if (error) return <Message text={errorText(error) ?? t("microfrontend.loadFailed")} />;
  if (!microfrontend?.url) return <Message text={t("microfrontend.noUrl")} />;

  const props: RemotePageProps = {
    activationKey,
    microfrontendActivationKey: activationKey,
    environment,
    i18n,
    /*
     * Ось http хоста — с токеном, окружением и обновлением пары.
     * Отличие от старой админки, о котором ремоут стоит предупредить:
     * наш перехватчик СНИМАЕТ конверт ответа (`unwrap` в shared/api/client),
     * то есть в `response.data` лежит уже полезная часть, а не
     * `{data: …, status: …}`.
     */
    sharedHttpRequest: http,
    sharedHttpRequestV2: httpAuth,
    params,
  };

  return (
    <RemoteBoundary key={activationKey} fallback={t("microfrontend.loadFailed")}>
      <Suspense fallback={<Message text={t("common.loading")} />}>
        {Page && <Page {...props} />}
      </Suspense>
    </RemoteBoundary>
  );
}

/**
 * Окружение одним словом, как его ждёт ремоут: всё, что не `production`,
 * для него `staging`. Так же считала и старая админка
 * (`MicrofrontendComponent/index.jsx`).
 */
function useEnvironmentKind(): "production" | "staging" {
  const projectId = useSession().getProjectId() ?? "";
  const environmentId = useSession().getEnvironmentId() ?? "";
  const { data } = useEnvironments(projectId);

  const name = data?.find((item) => item.id === environmentId)?.name ?? "";
  return name.toLowerCase() === "production" ? "production" : "staging";
}

function Message({ text }: { text: string }) {
  return (
    <div className="grid flex-1 place-items-center p-8 text-center">
      <p className="max-w-sm text-sm text-fg-muted">{text}</p>
    </div>
  );
}

/**
 * Граница ошибки вокруг ремоута.
 *
 * Без неё сбой в чужом коде — а он чужой и собран отдельно — снимает
 * с экрана всю админку: React размонтирует дерево до корня. Здесь же
 * падение остаётся внутри пункта меню, и сайдбар с шапкой стоят на месте.
 */
class RemoteBoundary extends Component<
  { children: ReactNode; fallback: string },
  { failed: boolean }
> {
  override state = { failed: false };

  static getDerivedStateFromError() {
    return { failed: true };
  }

  override componentDidCatch(error: unknown) {
    // Причина нужна тому, кто собирал ремоут: несовпавшая версия,
    // отсутствующий `./Page`, CORS. В интерфейс её не выносим —
    // читать её всё равно по стеку.
    console.error("microfrontend failed", error);
  }

  override render() {
    return this.state.failed ? <Message text={this.props.fallback} /> : this.props.children;
  }
}

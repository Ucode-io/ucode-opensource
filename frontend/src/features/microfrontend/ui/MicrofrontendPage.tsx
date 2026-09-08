import { Suspense, lazy, useEffect, useMemo, type ComponentType } from "react";
import { useTranslation } from "react-i18next";
import { useEnvironments } from "@/features/workspace";
import { errorText } from "@/shared/api/client";
import { useSession } from "@/shared/api/use-session";
import i18n from "@/shared/lib/i18n";
import { useMicrofrontend } from "../api/microfrontend";
import { remoteHttp, remoteHttpV2 } from "../api/remote-http";
import { startLegacyMirror } from "../model/legacy-mirror";
import { pinRootFontSize, toggleRemoteStyles } from "../model/remote-styles";
import {
  RemoteContractError,
  entryUrl,
  loadRemotePage,
  type RemotePageProps,
} from "../model/remote";
import { RemoteBoundary } from "./RemoteBoundary";
import { RemoteHost } from "./RemoteHost";

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
  const environment = useEnvironment();

  /*
   * Ремоут перемонтируется на каждый заход, а не оживает с прежним
   * состоянием: он держит своё дерево целиком, и показать чужой экран
   * с данными прошлого пункта хуже, чем загрузить его заново. Старая
   * админка делала то же самое ключом `activationKey`.
   */
  const activationKey = `${id}:${microfrontend?.url ?? ""}`;

  /*
   * Зеркало сессии — для ремоутов, которые ходят в API своим axios,
   * а не переданной осью. Пишет его загрузчик и только для старого
   * поколения; здесь мы лишь держим его свежим, пока экран открыт,
   * и стираем на выходе. Почему так — в model/legacy-mirror.
   */
  useEffect(() => startLegacyMirror(), []);

  /*
   * Событие захода. Старая админка стреляла им при каждом открытии
   * (`MicrofrontendComponent/index.jsx:30-38`), и ремоут, подписанный
   * на него, иначе не узнает о повторном заходе: пропсы читают не все.
   */
  useEffect(() => {
    const link = entryUrl(microfrontend?.url ?? "");
    if (!link) return;

    window.dispatchEvent(
      new CustomEvent("ucode:microfrontend:activate", { detail: { activationKey, link } }),
    );
  }, [activationKey, microfrontend?.url]);

  /*
   * Стили ремоута включаем на входе и гасим на выходе: плагин федерации
   * вставляет их в наш `<head>` один раз и навсегда, а в них бывает
   * `html { font-size: … }` — после одного захода вся админка оставалась
   * мельче до перезагрузки.
   *
   * Размер корня закрепляем ЗДЕСЬ, а не в `RemoteHost`: этот эффект
   * успевает до того, как приедут чужие чанки (а это мегабайты), тогда
   * как `RemoteHost` монтируется уже после них. Иначе админка на всё
   * время загрузки съезжала бы и потом прыгала обратно.
   */
  useEffect(() => {
    const entry = entryUrl(microfrontend?.url ?? "");
    if (!entry) return;

    const unpin = pinRootFontSize();
    toggleRemoteStyles(entry, true);

    return () => {
      unpin();
      toggleRemoteStyles(entry, false);
    };
  }, [microfrontend?.url]);

  const Page = useMemo(() => {
    const url = microfrontend?.url;
    if (!url) return null;

    /*
     * `lazy` держит на себе только ЗАГРУЗКУ: пока сборка едет —
     * `Suspense`, не доехала — граница ошибки.
     *
     * Дальше две ветки, и выбирает их сам ремоут своим `./meta`.
     * Новый собран против нашего React 19 — он обычный компонент
     * в нашем дереве. Старый работает на своём React 18, и ему нужен
     * отдельный корень.
     */
    return lazy(async () => {
      const { page, meta } = await loadRemotePage(id, url, environment.name);

      if (meta) return { default: page as ComponentType<RemotePageProps> };
      return { default: (props: RemotePageProps) => <RemoteHost page={page} props={props} /> };
    });
  }, [id, microfrontend?.url, environment.name]);

  // Пункт есть, приложение к нему не привязано: у бэкенда `microfrontend_id`
  // необязателен, а сменить его правкой пункта нельзя — только завести заново.
  if (!id) return <Message text={t("microfrontend.notPicked")} />;
  if (isLoading) return <Message text={t("common.loading")} />;
  if (error) return <Message text={errorText(error) ?? t("microfrontend.loadFailed")} />;
  if (!microfrontend?.url) return <Message text={t("microfrontend.noUrl")} />;

  const props: RemotePageProps = {
    activationKey,
    microfrontendActivationKey: activationKey,
    environment: environment.kind,
    i18n,
    /*
     * Оси http для ремоута — отдельные, не наши `http`/`httpAuth`:
     * контракт у них старый, чужой, и живёт он в api/remote-http.
     */
    sharedHttpRequest: remoteHttp,
    sharedHttpRequestV2: remoteHttpV2,
    params,
  };

  return (
    <RemoteBoundary
      key={activationKey}
      fallback={(failure) => (
        <Message
          text={
            failure instanceof RemoteContractError
              ? t("microfrontend.tooNew")
              : t("microfrontend.loadFailed")
          }
        />
      )}
    >
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
function useEnvironment(): { name: string; kind: "production" | "staging" } {
  const projectId = useSession().getProjectId() ?? "";
  const environmentId = useSession().getEnvironmentId() ?? "";
  const { data } = useEnvironments(projectId);

  const name = data?.find((item) => item.id === environmentId)?.name ?? "";
  return { name, kind: name.toLowerCase() === "production" ? "production" : "staging" };
}

function Message({ text }: { text: string }) {
  return (
    <div className="grid flex-1 place-items-center p-8 text-center">
      <p className="max-w-sm text-sm text-fg-muted">{text}</p>
    </div>
  );
}

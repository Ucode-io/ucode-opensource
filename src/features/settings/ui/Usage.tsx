import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Tabs } from "@/shared/ui/tabs";
import {
  useUsage,
  useUsageActors,
  useUsageTimes,
  type UsageActor,
  type UsageRow,
} from "../api/usage";
import { Empty, SectionHeader, Td, Th } from "./parts";

/**
 * Расход API-запросов: сколько осталось от месячного лимита и какие
 * маршруты его съедают.
 *
 * Это не биллинг (тарифы выброшены — PARITY.md, раздел Sidebar):
 * здесь не деньги, а потолок, в который проект упирается, и ответ
 * на «откуда сто тысяч запросов». Поэтому — вкладка журнала, рядом
 * с изменениями и функциями, а не собственный раздел.
 *
 * Три уровня, каждый — своя таблица: маршруты → отправители маршрута →
 * время отправителя. Клик по строке ведёт глубже, «Назад» — обратно.
 * Табов по разрезам и чипов фильтров нет намеренно: в них вопрос
 * «кто ест лимит» тонет — обе ранние версии экрана это показали.
 *
 * Срез «только клиентское API» — главный переключатель: админский
 * трафик (клики по билдеру, включая сам этот экран) лимит расходует,
 * но не блокируется никогда — режется только клиентское API.
 */
export function Usage() {
  const { t, i18n } = useTranslation();
  const [scope, setScope] = useState("all");
  // Путь вглубь: выбранный маршрут, затем выбранный отправитель.
  const [route, setRoute] = useState<UsageRow | null>(null);
  const [sender, setSender] = useState<UsageActor | null>(null);

  const clientOnly = scope === "client";
  const view = sender ? "time" : route ? "actor" : "route";

  // Сводка сверху всегда с уровня маршрутов — кэш живёт, лишних запросов нет.
  const { usage, isLoading } = useUsage(clientOnly);
  const { actors, isLoading: actorsLoading } = useUsageActors(route, clientOnly);
  const { times, isLoading: timesLoading } = useUsageTimes(route, sender, clientOnly);

  const amount = (value: number) => value.toLocaleString(i18n.language);

  const authLabel = (authType: string) =>
    authType === "bearer"
      ? t("usage.authBearer")
      : authType === "api_key"
        ? t("usage.authApiKey")
        : "—";

  /*
   * Без auth — служебный трафик, автора у него не бывает.
   * С auth, но без имени — автор был, но запись его не сохранила.
   */
  const senderName = (actor: UsageActor) =>
    actor.name || (actor.authType ? t("usage.senderUnknown") : t("usage.senderNoAuth"));

  const timeLabel = (bucket: string) => {
    const date = new Date(bucket);
    return Number.isNaN(date.getTime())
      ? bucket
      : date.toLocaleString(i18n.language, {
          month: "2-digit",
          day: "2-digit",
          hour: "2-digit",
          minute: "2-digit",
        });
  };

  const back = () => (sender ? setSender(null) : setRoute(null));

  // Без auth и id фильтровать не по чему — такая строка не раскрывается.
  const canDrill = (actor: UsageActor) => Boolean(actor.actorId || actor.authType);

  const viewLoading =
    view === "route" ? isLoading : view === "actor" ? actorsLoading : timesLoading;
  const viewLength = view === "route" ? (usage?.top.length ?? 0) : view === "actor" ? actors.length : times.length;
  const columns = view === "route" ? 5 : view === "actor" ? 4 : 3;

  return (
    <div className="flex min-h-0 min-w-0 flex-1 flex-col">
      <SectionHeader title={t("usage.title")} hint={t("usage.hint")} />

      {usage && (
        <div className="shrink-0 border-b border-border px-4 py-3">
          {usage.blocked && <p className="mb-1 text-sm text-danger">{t("usage.blocked")}</p>}
          <p
            className={`text-sm ${
              (usage.percentUsed ?? 0) > 80 && !usage.blocked ? "text-warning" : "text-fg"
            }`}
          >
            {usage.unlimited
              ? t("usage.unlimited", { used: amount(usage.used) })
              : t("usage.used", { used: amount(usage.used), limit: amount(usage.limit) })}
          </p>
        </div>
      )}

      <div className="shrink-0 border-b border-border px-4 py-2">
        <Tabs
          tabs={[
            { id: "all", label: t("usage.allTraffic") },
            { id: "client", label: t("usage.clientOnly") },
          ]}
          activeId={scope}
          onSelect={setScope}
        />
      </div>

      {/* Где мы: выбранный маршрут (и отправитель) вместо чипов фильтров. */}
      {route && (
        <div className="flex shrink-0 items-center gap-2 border-b border-border px-4 py-2">
          <button
            type="button"
            onClick={back}
            className="shrink-0 text-xs font-medium text-accent-text hover:underline"
          >
            ← {t("usage.back")}
          </button>
          <span className="truncate text-xs text-fg-muted">
            {route.label}
            {sender ? ` · ${senderName(sender)}` : ""}
            {` — ${amount((sender ?? route).count)}`}
          </span>
        </div>
      )}

      <div className="min-h-0 flex-1 overflow-auto">
        <table className="w-full border-separate border-spacing-0">
          <thead>
            <tr>
              {view === "route" && (
                <>
                  <Th className="w-28">{t("usage.source")}</Th>
                  <Th>{t("usage.route")}</Th>
                  <Th className="w-32">{t("usage.table")}</Th>
                </>
              )}
              {view === "actor" && (
                <>
                  <Th className="w-28">{t("usage.auth")}</Th>
                  <Th>{t("usage.sender")}</Th>
                </>
              )}
              {view === "time" && <Th>{t("usage.time")}</Th>}
              <Th className="w-28 text-right">{t("usage.count")}</Th>
              <Th className="w-20 text-right">{t("usage.share")}</Th>
            </tr>
          </thead>

          <tbody>
            {viewLoading && <Empty text={t("common.loading")} colSpan={columns} />}
            {!viewLoading && !viewLength && <Empty text={t("usage.empty")} colSpan={columns} />}

            {!viewLoading &&
              view === "route" &&
              (usage?.top ?? []).map((row) => (
                <tr
                  key={`${row.source} ${row.label}`}
                  onClick={() => setRoute(row)}
                  className="cursor-pointer hover:bg-surface-hover"
                >
                  <Td className="text-fg-muted">
                    {row.source === "admin" ? t("usage.sourceAdmin") : t("usage.sourceClient")}
                  </Td>
                  <Td className="font-mono text-xs">{row.label}</Td>
                  <Td className="text-fg-muted">{row.collection || "—"}</Td>
                  <Td className="text-right tabular-nums">{amount(row.count)}</Td>
                  <Td className="text-right tabular-nums text-fg-muted">{row.percent}%</Td>
                </tr>
              ))}

            {!viewLoading &&
              view === "actor" &&
              actors.map((actor, index) => (
                <tr
                  key={index}
                  onClick={canDrill(actor) ? () => setSender(actor) : undefined}
                  className={canDrill(actor) ? "cursor-pointer hover:bg-surface-hover" : ""}
                >
                  <Td className="text-fg-muted">{authLabel(actor.authType)}</Td>
                  <Td>{senderName(actor)}</Td>
                  <Td className="text-right tabular-nums">{amount(actor.count)}</Td>
                  <Td className="text-right tabular-nums text-fg-muted">{actor.percent}%</Td>
                </tr>
              ))}

            {!viewLoading &&
              view === "time" &&
              times.map((time) => (
                <tr key={time.bucket}>
                  <Td className="tabular-nums">{timeLabel(time.bucket)}</Td>
                  <Td className="text-right tabular-nums">{amount(time.count)}</Td>
                  <Td className="text-right tabular-nums text-fg-muted">{time.percent}%</Td>
                </tr>
              ))}

            {/*
              «Прочее» — хвост за пределами десятки плюс трафик до выката
              разбивки. Раскрывать его нечем, поэтому не кликабельно.
            */}
            {!viewLoading && view === "route" && usage && usage.other > 0 && (
              <tr>
                <Td className="text-fg-muted" colSpan={columns - 2}>
                  {t("usage.other")}
                </Td>
                <Td className="text-right tabular-nums">{amount(usage.other)}</Td>
                <Td />
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}

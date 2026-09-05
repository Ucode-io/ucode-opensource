import { Fragment, useState } from "react";
import { IconChevronDown, IconChevronRight } from "@tabler/icons-react";
import { useTranslation } from "react-i18next";
import { Icon } from "@/shared/ui/icon";
import { Tabs } from "@/shared/ui/tabs";
import { useUsage, useUsageActors, type UsageRow } from "../api/usage";
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
 * Один экран без видов и фильтров: таблица маршрутов, строка
 * раскрывается и показывает, кто вызывал. Ручка умеет больше
 * (group_by по таблицам и времени, произвольные фильтры), но каждый
 * лишний разрез — это вкладки и чипы, в которых вопрос «кто ест
 * лимит» тонет; ровно так случилось с первой версией этого экрана.
 *
 * Срез «только клиентское API» — главный переключатель: админский
 * трафик (клики по билдеру, включая сам этот экран) лимит расходует,
 * но не блокируется никогда — режется только клиентское API.
 */
export function Usage() {
  const { t, i18n } = useTranslation();
  const [scope, setScope] = useState("all");
  const [opened, setOpened] = useState<Record<string, boolean>>({});

  const clientOnly = scope === "client";
  const { usage, isLoading } = useUsage(clientOnly);
  const rows = usage?.top ?? [];

  const amount = (value: number) => value.toLocaleString(i18n.language);

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

      <div className="min-h-0 flex-1 overflow-auto">
        <table className="w-full border-separate border-spacing-0">
          <thead>
            <tr>
              <Th className="w-28">{t("usage.source")}</Th>
              <Th>{t("usage.route")}</Th>
              <Th className="w-28 text-right">{t("usage.count")}</Th>
              <Th className="w-20 text-right">{t("usage.share")}</Th>
            </tr>
          </thead>

          <tbody>
            {isLoading && <Empty text={t("common.loading")} colSpan={4} />}
            {!isLoading && !rows.length && <Empty text={t("usage.empty")} colSpan={4} />}

            {rows.map((row) => {
              // Источник входит в ключ: один маршрут виден с обеих сторон.
              const key = `${row.source} ${row.label}`;
              const open = Boolean(opened[key]);

              return (
                <Fragment key={key}>
                  <tr
                    onClick={() => setOpened((state) => ({ ...state, [key]: !open }))}
                    className="cursor-pointer hover:bg-surface-hover"
                  >
                    <Td className="text-fg-muted">
                      <span className="flex items-center gap-1">
                        <Icon as={open ? IconChevronDown : IconChevronRight} size={14} />
                        {row.source === "admin" ? t("usage.sourceAdmin") : t("usage.sourceClient")}
                      </span>
                    </Td>
                    <Td className="font-mono text-xs">{row.label}</Td>
                    <Td className="text-right tabular-nums">{amount(row.count)}</Td>
                    <Td className="text-right tabular-nums text-fg-muted">{row.percent}%</Td>
                  </tr>

                  {open && <RouteSenders row={row} clientOnly={clientOnly} />}
                </Fragment>
              );
            })}

            {/*
              «Прочее» — хвост за пределами десятки плюс трафик до выката
              разбивки. Маршрута у него нет — раскрывать нечего.
            */}
            {usage && usage.other > 0 && (
              <tr>
                <Td className="text-fg-muted">{t("usage.other")}</Td>
                <Td />
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

/**
 * Кто вызывал раскрытый маршрут. Свой компонент — свой запрос: он уходит
 * только при раскрытии строки, закрытые строки ничего не тянут.
 *
 * Доли здесь — от запросов этого маршрута, а не от всего месяца:
 * так их отдаёт ручка под фильтром, и так они отвечают на вопрос
 * «кто из вызывающих главный».
 */
function RouteSenders({ row, clientOnly }: { row: UsageRow; clientOnly: boolean }) {
  const { t, i18n } = useTranslation();
  const { actors, isLoading } = useUsageActors(row, clientOnly);

  if (isLoading) {
    return (
      <tr>
        <Td />
        <Td className="text-xs text-fg-muted">{t("common.loading")}</Td>
        <Td />
        <Td />
      </tr>
    );
  }

  return (
    <>
      {actors.map((actor, index) => {
        const auth =
          actor.authType === "bearer"
            ? t("usage.authBearer")
            : actor.authType === "api_key"
              ? t("usage.authApiKey")
              : "";
        const label =
          auth && actor.name
            ? `${auth} · ${actor.name}`
            : auth || actor.name || t("usage.senderUnknown");

        return (
          <tr key={index} className="bg-bg">
            <Td />
            <Td className="text-xs text-fg-muted">{label}</Td>
            <Td className="text-right text-xs tabular-nums text-fg-muted">
              {actor.count.toLocaleString(i18n.language)}
            </Td>
            <Td className="text-right text-xs tabular-nums text-fg-muted">{actor.percent}%</Td>
          </tr>
        );
      })}
    </>
  );
}

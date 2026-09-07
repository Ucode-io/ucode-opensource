import { Fragment, useState } from "react";
import { IconChevronDown, IconChevronRight } from "@tabler/icons-react";
import { useTranslation } from "react-i18next";
import { Icon } from "@/shared/ui/icon";
import { Tabs } from "@/shared/ui/tabs";
import { useUsage, useUsageActors, useUsageTimeline, type UsageRow } from "../api/usage";
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
 * Один экран без видов и фильтров: таблица маршрутов, клик по строке
 * раскрывает панель деталей — кто вызывал и когда. Ручка умеет больше
 * (произвольные group_by и фильтры), но каждый лишний разрез — это
 * вкладки и чипы, в которых вопрос «кто ест лимит» тонет; ровно так
 * случилось с первой версией этого экрана.
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
          {usage.percentUsed !== null && (
            <div className="mt-2 h-1.5 max-w-xs overflow-hidden rounded-full bg-surface-active">
              <div
                className={`h-full rounded-full ${
                  usage.blocked
                    ? "bg-danger"
                    : usage.percentUsed > 80
                      ? "bg-warning"
                      : "bg-accent-solid"
                }`}
                style={{ width: `${usage.percentUsed}%` }}
              />
            </div>
          )}
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
              <Th className="w-36 text-right">{t("usage.share")}</Th>
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
                    <Td>
                      <span className="flex items-center justify-end gap-2">
                        <ShareBar percent={row.percent} />
                        <span className="w-11 text-right text-xs tabular-nums text-fg-muted">
                          {row.percent}%
                        </span>
                      </span>
                    </Td>
                  </tr>

                  {open && <RouteDetails row={row} clientOnly={clientOnly} />}
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

/** Полоска доли рядом с числом: число читается, полоска сравнивается. */
function ShareBar({ percent }: { percent: number }) {
  return (
    <span className="h-1 w-16 shrink-0 overflow-hidden rounded-full bg-surface-active">
      <span
        className="block h-full rounded-full bg-accent"
        style={{ width: `${Math.min(100, percent)}%` }}
      />
    </span>
  );
}

/**
 * Панель деталей раскрытого маршрута: кто вызывал и когда.
 * Оба блока грузятся только при раскрытии — закрытые строки не тянут.
 */
function RouteDetails({ row, clientOnly }: { row: UsageRow; clientOnly: boolean }) {
  const { t } = useTranslation();

  return (
    <tr className="bg-bg">
      <td colSpan={4} className="border-b border-border px-4 py-3">
        <div className="flex flex-wrap gap-x-10 gap-y-4">
          <div className="min-w-64 max-w-md flex-1">
            <h4 className="mb-2 text-xs font-medium text-fg-muted">{t("usage.senders")}</h4>
            <RouteSenders row={row} clientOnly={clientOnly} />
          </div>
          <div className="min-w-64 max-w-md flex-1">
            <h4 className="mb-2 text-xs font-medium text-fg-muted">{t("usage.activity")}</h4>
            <RouteActivity row={row} clientOnly={clientOnly} />
          </div>
        </div>
      </td>
    </tr>
  );
}

/**
 * Кто вызывал раскрытый маршрут.
 *
 * Доли здесь — от запросов этого маршрута, а не от всего месяца:
 * так их отдаёт ручка под фильтром, и так они отвечают на вопрос
 * «кто из вызывающих главный».
 */
function RouteSenders({ row, clientOnly }: { row: UsageRow; clientOnly: boolean }) {
  const { t, i18n } = useTranslation();
  const { actors, isLoading } = useUsageActors(row, clientOnly);

  if (isLoading) return <p className="text-xs text-fg-subtle">{t("common.loading")}</p>;
  if (!actors.length) return <p className="text-xs text-fg-subtle">{t("usage.empty")}</p>;

  return (
    <ul className="flex flex-col gap-1.5">
      {actors.map((actor, index) => {
        const auth =
          actor.authType === "bearer"
            ? t("usage.authBearer")
            : actor.authType === "api_key"
              ? t("usage.authApiKey")
              : "";
        /*
         * Без auth — служебный трафик, автора у него не бывает.
         * С auth, но без имени — автор был, но запись его не сохранила:
         * говорим и то, и другое, а не одно слово «User».
         */
        const label = !auth
          ? t("usage.senderNoAuth")
          : actor.name
            ? `${auth} · ${actor.name}`
            : `${auth} — ${t("usage.senderUnknown")}`;

        return (
          <li key={index} className="flex items-center gap-2">
            <span className="min-w-0 flex-1 truncate text-xs text-fg">{label}</span>
            <span className="text-xs tabular-nums text-fg-muted">
              {actor.count.toLocaleString(i18n.language)}
            </span>
            <ShareBar percent={actor.percent} />
            <span className="w-11 shrink-0 text-right text-xs tabular-nums text-fg-muted">
              {actor.percent}%
            </span>
          </li>
        );
      })}
    </ul>
  );
}

/** Когда вызывали раскрытый маршрут: запросы по дням с начала месяца. */
function RouteActivity({ row, clientOnly }: { row: UsageRow; clientOnly: boolean }) {
  const { t, i18n } = useTranslation();
  const { days, isLoading } = useUsageTimeline(row, clientOnly);

  const first = days[0];
  const last = days[days.length - 1];

  if (isLoading) return <p className="text-xs text-fg-subtle">{t("common.loading")}</p>;
  if (!first || !last) return <p className="text-xs text-fg-subtle">{t("usage.empty")}</p>;

  const max = Math.max(...days.map((day) => day.count));
  const dayLabel = (day: string) =>
    // Ведро в UTC — без timeZone браузер утащит дату на сутки назад.
    new Date(`${day}T00:00:00Z`).toLocaleDateString(i18n.language, {
      day: "numeric",
      month: "short",
      timeZone: "UTC",
    });

  return (
    <div>
      <div className="flex h-16 items-end gap-px">
        {days.map((day) => (
          <div
            key={day.day}
            title={`${dayLabel(day.day)} — ${day.count.toLocaleString(i18n.language)}`}
            className={`min-w-1 flex-1 rounded-t-xs ${day.count ? "bg-accent" : "bg-surface-active"}`}
            style={{ height: day.count ? `${Math.max(8, (day.count / max) * 100)}%` : "2px" }}
          />
        ))}
      </div>
      <div className="mt-1 flex justify-between text-[10px] text-fg-subtle">
        <span>{dayLabel(first.day)}</span>
        {days.length > 1 && <span>{dayLabel(last.day)}</span>}
      </div>
    </div>
  );
}

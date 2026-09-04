import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Tabs } from "@/shared/ui/tabs";
import { useUsage } from "../api/usage";
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
 * Переключатель «только клиентский» фильтрует на фронте: параметра
 * source у ручки нет. Он тут главный: админский трафик (клики
 * по билдеру, включая сам этот экран) лимит расходует, но не
 * блокируется никогда — режется только клиентское API.
 */
export function Usage() {
  const { t, i18n } = useTranslation();
  const [scope, setScope] = useState("all");
  const { usage, isLoading } = useUsage();

  const clientOnly = scope === "client";
  const rows = usage
    ? clientOnly
      ? usage.top.filter((row) => row.source === "client")
      : usage.top
    : [];

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

            {rows.map((row) => (
              // Источник входит в ключ: один маршрут виден с обеих сторон.
              <tr key={`${row.source} ${row.label}`}>
                <Td className="text-fg-muted">
                  {row.source === "admin" ? t("usage.sourceAdmin") : t("usage.sourceClient")}
                </Td>
                <Td className="font-mono text-xs">{row.label}</Td>
                <Td className="text-right tabular-nums">{amount(row.count)}</Td>
                <Td className="text-right tabular-nums text-fg-muted">{row.percent}%</Td>
              </tr>
            ))}

            {/*
              «Прочее» — хвост за пределами десятки плюс трафик до выката
              разбивки. Источника у него нет, поэтому в клиентском срезе
              строка не рисуется, а не притворяется клиентской.
            */}
            {!clientOnly && usage && usage.other > 0 && (
              <tr>
                <Td />
                <Td className="text-fg-muted">{t("usage.other")}</Td>
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

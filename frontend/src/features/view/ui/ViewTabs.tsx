import { useMemo } from "react";
import { useTranslation } from "react-i18next";
import type { TranslationKey } from "@/shared/lib/i18n";
import { Tabs } from "@/shared/ui/tabs";
import { viewName, type View } from "../model/types";
import { viewIcon } from "./view-icon";

/**
 * Переключатель view — та же полоса вкладок, что и везде (shared/ui/tabs).
 *
 * Показывается и с одной вкладкой: это единственное место, где видно имя
 * открытого view, и без него страница с одним view выглядела как страница
 * без view вообще. Скрывать полосу было ошибкой.
 */
export function ViewTabs({
  views,
  activeId,
  language,
  onSelect,
}: {
  views: View[];
  activeId: string;
  /** Язык ДАННЫХ: имена view хранятся на языках проекта, как и подписи полей. */
  language: string;
  onSelect: (view: View) => void;
}) {
  const { t } = useTranslation();

  /*
   * Имя вкладки: заданное админом на языке данных, иначе на базовом,
   * иначе тип. Безымянных view большинство, и «Таблица» рядом
   * с «Доской» читается лучше, чем пустая вкладка.
   */
  const tabs = useMemo(
    () =>
      views.map((view) => ({
        id: view.id,
        label:
          viewName(view, language) ||
          t(`view.type.${view.type}` as TranslationKey, { defaultValue: view.type }),
        icon: viewIcon(view.type),
      })),
    [views, language, t],
  );

  return (
    <Tabs
      tabs={tabs}
      activeId={activeId}
      onSelect={(id) => {
        const view = views.find((item) => item.id === id);
        if (view) onSelect(view);
      }}
    />
  );
}

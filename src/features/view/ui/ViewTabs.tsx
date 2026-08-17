import { useTranslation } from "react-i18next";
import type { TranslationKey } from "@/shared/lib/i18n";
import { Icon } from "@/shared/ui/icon";
import { viewName, type View } from "../model/types";
import { viewIcon } from "./view-icon";

/**
 * Переключатель view — сегментированный контрол, как вкладки списка
 * и доски в референсе.
 *
 * Показывается и с одной вкладкой: это единственное место, где видно имя
 * открытого view, и без него страница с одним view выглядела как страница
 * без view вообще. Скрывать полосу было ошибкой.
 *
 * Много вкладок — полоса прокручивается вбок. В старой админке вместо
 * этого мерили ширину и складывали лишнее в меню «Ещё», из-за чего
 * последняя видимая вкладка переставала открываться и превращалась
 * в кнопку меню. Прокрутка стоит одного класса и не врёт.
 */
const tab =
  "inline-flex h-7 shrink-0 items-center gap-1.5 rounded-md px-2 text-sm transition-colors";

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

  if (!views.length) return null;

  return (
    <div className="flex min-w-0 items-center gap-0.5 overflow-x-auto rounded-lg bg-surface-active p-0.5">
      {views.map((view) => {
        const active = view.id === activeId;
        /*
         * Имя вкладки: заданное админом на языке данных, иначе на базовом,
         * иначе тип. Безымянных view большинство, и «Таблица» рядом
         * с «Доской» читается лучше, чем пустая вкладка.
         */
        const type = t(`view.type.${view.type}` as TranslationKey, { defaultValue: view.type });

        return (
          <button
            key={view.id}
            type="button"
            aria-current={active ? "page" : undefined}
            onClick={() => onSelect(view)}
            className={`${tab} whitespace-nowrap ${
              active ? "bg-surface text-fg shadow-raised" : "text-fg-muted hover:text-fg"
            }`}
          >
            <Icon as={viewIcon(view.type)} size={14} />
            {viewName(view, language) || type}
          </button>
        );
      })}
    </div>
  );
}

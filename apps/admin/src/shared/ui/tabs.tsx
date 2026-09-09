import type { Icon as TablerIcon } from "@tabler/icons-react";
import { Icon } from "@/shared/ui/icon";

/**
 * Вкладки — сегментированный контрол: дорожка, а в ней плашка активной.
 *
 * Один вид на все вкладки приложения. Ими показывается не только набор
 * view: раскладка таблицы по значению поля (`group_fields`) и связи
 * в карточке записи — это те же вкладки, с тем же поведением. В старой
 * админке на каждый случай был свой компонент со своими кнопками
 * (Grid.jsx, tabGroupBtn), и три полосы вкладок выглядели по-разному
 * без единой причины; у нас это успело повториться в карточке.
 *
 * Живёт в shared, а не в features/view: карточку записи рисует
 * features/item, а обратный импорт (item → view) замкнул бы фичи
 * в кольцо — features/view уже импортирует features/item.
 *
 * Много вкладок — полоса прокручивается вбок. В старой админке вместо
 * этого мерили ширину и складывали лишнее в меню «Ещё», из-за чего
 * последняя видимая вкладка переставала открываться и превращалась
 * в кнопку меню. Прокрутка стоит одного класса и не врёт.
 */
export type TabItem = {
  id: string;
  label: string;
  icon?: TablerIcon;
  /** Подсказка под курсором. Нужна там, где подпись сокращена до кода. */
  title?: string;
};

const tab =
  "inline-flex h-7 shrink-0 items-center gap-1.5 rounded-md px-2 text-sm transition-colors";

export function Tabs({
  tabs,
  activeId,
  onSelect,
}: {
  tabs: TabItem[];
  activeId: string;
  onSelect: (id: string) => void;
}) {
  if (!tabs.length) return null;

  return (
    // Дорожка по содержимому, а не во всю строку: три вкладки на всю
    // ширину модалки читаются как пустая панель, у которой что-то
    // не загрузилось. `max-w-full` оставляет прокрутку там, где вкладок
    // больше, чем помещается.
    <div className="flex w-fit min-w-0 max-w-full items-center gap-0.5 overflow-x-auto rounded-lg bg-surface-active p-0.5">
      {tabs.map((item) => {
        const active = item.id === activeId;

        return (
          <button
            key={item.id}
            type="button"
            aria-current={active ? "page" : undefined}
            onClick={() => onSelect(item.id)}
            {...(item.title ? { title: item.title } : {})}
            className={`${tab} whitespace-nowrap ${
              active ? "bg-surface text-fg shadow-raised" : "text-fg-muted hover:text-fg"
            }`}
          >
            {item.icon && <Icon as={item.icon} size={14} />}
            {item.label}
          </button>
        );
      })}
    </div>
  );
}

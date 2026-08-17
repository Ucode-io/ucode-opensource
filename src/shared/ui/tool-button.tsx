import { Icon } from "@/shared/ui/icon";
import type { Icon as TablerIcon } from "@tabler/icons-react";

/**
 * Кнопка-иконка в строке над таблицей: фильтр, сортировка, поиск,
 * настройки view.
 *
 * Две независимые подсветки, и это не украшение.
 *
 *   open — панель раскрыта: подложка. Она говорит «вот куда ты смотришь».
 *   on   — отбор задан: цвет самой иконки. Он говорит «список неполный».
 *
 * Складывать их в одно «активно» нельзя: закрытая панель с заданным
 * фильтром выглядела бы раскрытой, а раскрытая пустая — как будто
 * что-то отфильтровано.
 */
export function ToolButton({
  icon,
  label,
  open = false,
  on = false,
  onClick,
}: {
  icon: TablerIcon;
  label: string;
  open?: boolean;
  on?: boolean;
  onClick: () => void;
}) {
  const color = on ? "text-accent-text" : open ? "text-fg" : "text-fg-muted hover:text-fg";

  return (
    <button
      type="button"
      onClick={onClick}
      aria-label={label}
      aria-pressed={open}
      title={label}
      className={`grid size-7 shrink-0 place-items-center rounded-md transition-colors ${color} ${
        open ? "bg-accent-subtle" : "hover:bg-surface-hover"
      }`}
    >
      <Icon as={icon} size={16} />
    </button>
  );
}

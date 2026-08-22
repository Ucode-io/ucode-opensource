import { IconCheck, IconChevronDown } from "@tabler/icons-react";
import { Icon } from "./icon";
import { Popover, PopoverItem } from "./popover";

export type DropdownItem = { value: string; label: string };

/**
 * Выпадающий список в нашем оформлении.
 *
 * Не `<select>`: системное меню браузера рисуется шрифтом и цветами
 * операционной системы, тёмную тему не знает и рядом с нашими кнопками
 * выглядит чужим. Внутри — тот же `Popover`, что у меню строки сайдбара
 * и у панели view, поэтому Escape, закрытие по клику мимо и переворот
 * вверх у него уже правильные.
 *
 * Отличие от `SelectMenu`: тот раскрывается НА МЕСТЕ и живёт внутри
 * панели поля — у него поиск и догрузка страниц. Здесь список короткий
 * и известен целиком, а место под кнопкой занято таблицей: раскрываться
 * он должен слоем поверх, не раздвигая экран.
 */
export function Dropdown({
  value,
  items,
  placeholder = "",
  ariaLabel,
  className = "",
  onChange,
}: {
  value: string;
  items: DropdownItem[];
  /** Что на кнопке, когда ничего не выбрано. */
  placeholder?: string;
  ariaLabel?: string;
  /** Ширина задаётся снаружи: у панели свои размеры. */
  className?: string;
  onChange: (value: string) => void;
}) {
  const chosen = items.find((item) => item.value === value);

  return (
    <Popover
      trigger={({ open, toggle }) => (
        <button
          type="button"
          onClick={toggle}
          aria-expanded={open}
          aria-haspopup="menu"
          {...(ariaLabel ? { "aria-label": ariaLabel } : {})}
          className={`flex h-(--spacing-input) w-full items-center gap-2 rounded-md border bg-surface px-2.5 text-left text-sm transition-colors ${
            open ? "border-accent" : "border-border-strong hover:border-fg-subtle"
          } ${className}`}
        >
          <span className={`flex-1 truncate ${chosen ? "text-fg" : "text-fg-subtle"}`}>
            {chosen?.label ?? placeholder}
          </span>
          <Icon
            as={IconChevronDown}
            size={14}
            className={`shrink-0 text-fg-subtle transition-transform ${open ? "rotate-180" : ""}`}
          />
        </button>
      )}
    >
      {(close) => (
        /* Список бывает длиннее экрана — прокручиваем его, а не страницу.
           Popover подкручивает себя в видимую часть, но высоту не
           ограничивает: это дело содержимого. */
        <div className="max-h-64 overflow-y-auto">
          {items.map((item) => (
            <PopoverItem
              key={item.value}
              active={item.value === value}
              {...(item.value === value
                ? { trailing: <Icon as={IconCheck} size={14} className="shrink-0" /> }
                : {})}
              onClick={() => {
                onChange(item.value);
                // Выбор одиночный: после него выбирать больше нечего.
                close();
              }}
            >
              {item.label}
            </PopoverItem>
          ))}
        </div>
      )}
    </Popover>
  );
}

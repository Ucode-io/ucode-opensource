import { useEffect, useRef, type ComponentProps } from "react";
import { IconCheck, IconMinus } from "@tabler/icons-react";

/**
 * Чекбокс. Внутри — нативный <input type="checkbox">, у которого снят
 * только внешний вид (appearance-none).
 *
 * Радиксовый вариант из shadcn тянет @radix-ui/react-checkbox ради того,
 * что нативный элемент уже умеет: фокус с клавиатуры, пробел, роль,
 * связь с <label>, состояние в форме, indeterminate. Здесь от него
 * нужен только внешний вид, а он — это классы.
 *
 * indeterminate живёт в DOM, а не в разметке: атрибута нет, только
 * свойство узла. Поэтому ref и эффект — это не обход React, это
 * единственный способ его выставить.
 */
export function Checkbox({
  indeterminate = false,
  className = "",
  ...props
}: Omit<ComponentProps<"input">, "type"> & { indeterminate?: boolean }) {
  const input = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (input.current) input.current.indeterminate = indeterminate;
  }, [indeterminate]);

  return (
    <span className="relative inline-grid size-4 shrink-0 place-items-center">
      <input
        {...props}
        ref={input}
        type="checkbox"
        className={`peer size-4 cursor-pointer appearance-none rounded-sm border border-border-strong bg-surface transition-colors checked:border-accent-solid checked:bg-accent-solid indeterminate:border-accent-solid indeterminate:bg-accent-solid disabled:cursor-default disabled:opacity-40 ${className}`}
      />

      {/*
        Галка и черта рисуются поверх и не ловят курсор — клик всегда
        достаётся самому input'у. Толщина 3: на 12px наша обычная 1.6
        превращается в еле заметную линию.
      */}
      <IconCheck
        aria-hidden
        stroke={3}
        className="pointer-events-none absolute hidden size-3 text-accent-fg peer-checked:block peer-indeterminate:hidden"
      />
      <IconMinus
        aria-hidden
        stroke={3}
        className="pointer-events-none absolute hidden size-3 text-accent-fg peer-indeterminate:block"
      />
    </span>
  );
}

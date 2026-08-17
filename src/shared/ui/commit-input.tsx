import { useState } from "react";
import { Input } from "./input";

/**
 * Поле ввода, которое отдаёт значение по Enter и по уходу фокуса,
 * а не на каждую букву.
 *
 * Всё, что здесь правят, уезжает на сервер запросом. Без этой задержки
 * переименование в семь букв — это семь PUT'ов, и последний обгоняет
 * предпоследний примерно всегда.
 *
 * Пустое значение не отправляется: стереть имя нельзя ни у view (бэкенд
 * обновляет колонку только непустым), ни у таблицы (безымянная строка
 * во всех списках). Пустое поле означает «передумал», и по уходу фокуса
 * в нём снова прежнее значение.
 */
export function CommitInput({
  value,
  placeholder,
  label,
  onCommit,
  ...rest
}: {
  value: string;
  placeholder?: string;
  /** Подпись для читалки экрана: видимой подписи у поля обычно нет. */
  label: string;
  onCommit: (value: string) => void;
  disabled?: boolean;
  className?: string;
}) {
  const [text, setText] = useState(value);

  const commit = () => {
    const next = text.trim();
    if (next && next !== value) onCommit(next);
    else if (!next) setText(value);
  };

  return (
    <Input
      {...rest}
      value={text}
      onChange={(event) => setText(event.target.value)}
      onBlur={commit}
      onKeyDown={(event) => {
        if (event.key === "Enter") {
          event.preventDefault();
          commit();
          event.currentTarget.blur();
        }
      }}
      {...(placeholder === undefined ? {} : { placeholder })}
      aria-label={label}
    />
  );
}

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
 * Пустое значение по умолчанию не отправляется: стереть имя нельзя ни
 * у view (бэкенд обновляет колонку только непустым), ни у таблицы
 * (безымянная строка во всех списках). Пустое поле означает «передумал»,
 * и по уходу фокуса в нём снова прежнее значение.
 *
 * `allowEmpty` — там, где пустота и есть значение: стёртый адрес перехода
 * означает «открывать карточку, как обычно», и запретить его нельзя.
 */
export function CommitInput({
  value,
  placeholder,
  label,
  allowEmpty = false,
  onCommit,
  ...rest
}: {
  value: string;
  placeholder?: string;
  /** Подпись для читалки экрана: видимой подписи у поля обычно нет. */
  label: string;
  /** Пустая строка — тоже значение, и её нужно отправить. */
  allowEmpty?: boolean;
  onCommit: (value: string) => void;
  disabled?: boolean;
  className?: string;
}) {
  const [text, setText] = useState(value);

  const commit = () => {
    const next = text.trim();
    if (next !== value && (next || allowEmpty)) onCommit(next);
    else if (!next && !allowEmpty) setText(value);
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

/**
 * Разметка ответа помощника.
 *
 * Модель отвечает markdown'ом: списком созданного, именами таблиц
 * в обратных кавычках, выделением. Показывать это как есть нельзя —
 * человек читает `**product**` вместе со звёздочками.
 *
 * Полный markdown здесь не нужен и не берётся пакетом: react-markdown
 * с remark-gfm — это 40 КБ парсера, из которого используются жирный,
 * код, список и абзац. Ровно они здесь и разобраны; всё остальное
 * (таблицы, заголовки, ссылки) остаётся текстом, а не ломается.
 */

export type Block =
  | { kind: "code"; text: string }
  | { kind: "list"; ordered: boolean; items: string[] }
  | { kind: "text"; text: string };

export type Token = { kind: "plain" | "bold" | "code"; text: string };

const FENCE = "```";
const BULLET = /^\s*[-*]\s+/;
const NUMBER = /^\s*\d+[.)]\s+/;

/** Разбирает ответ на блоки. Пустой текст — пустой список. */
export function blocks(source: string): Block[] {
  const lines = source.split("\n");
  const out: Block[] = [];

  /** Накопленные строки абзаца: он кончается пустой строкой или другим блоком. */
  let paragraph: string[] = [];
  const flush = () => {
    const text = paragraph.join("\n").trim();
    if (text) out.push({ kind: "text", text });
    paragraph = [];
  };

  for (let i = 0; i < lines.length; i++) {
    const line = lines[i]!;

    // Огороженный код: до закрывающей ограды или до конца текста —
    // модель нередко обрывает ответ, не закрыв её.
    if (line.trimStart().startsWith(FENCE)) {
      flush();
      const body: string[] = [];
      for (i++; i < lines.length && !lines[i]!.trimStart().startsWith(FENCE); i++) {
        body.push(lines[i]!);
      }
      // trimEnd, а не trim: отступ первой строки — часть кода.
      out.push({ kind: "code", text: body.join("\n").trimEnd() });
      continue;
    }

    const ordered = NUMBER.test(line);
    if (ordered || BULLET.test(line)) {
      flush();
      const items: string[] = [];
      const mark = ordered ? NUMBER : BULLET;
      for (; i < lines.length && mark.test(lines[i]!); i++) {
        items.push(lines[i]!.replace(mark, ""));
      }
      i--;
      out.push({ kind: "list", ordered, items });
      continue;
    }

    if (line.trim()) paragraph.push(line);
    else flush();
  }

  flush();
  return out;
}

/**
 * Разбирает строку на куски: обычный текст, жирный и код.
 *
 * Одним проходом по обоим маркерам, а не двумя вложенными: звёздочки
 * внутри `\`кода\`` — это звёздочки, а не выделение.
 */
export function inline(source: string): Token[] {
  const out: Token[] = [];
  let plain = "";

  const push = (kind: Token["kind"], text: string) => {
    if (text) out.push({ kind, text });
  };

  for (let i = 0; i < source.length; ) {
    const marker = source.startsWith("**", i) ? "**" : source[i] === "`" ? "`" : "";
    const end = marker ? source.indexOf(marker, i + marker.length) : -1;

    // Одинокий маркер без пары — обычный символ, а не начало выделения.
    if (!marker || end === -1) {
      plain += source[i];
      i += 1;
      continue;
    }

    push("plain", plain);
    plain = "";
    push(marker === "`" ? "code" : "bold", source.slice(i + marker.length, end));
    i = end + marker.length;
  }

  push("plain", plain);
  return out;
}

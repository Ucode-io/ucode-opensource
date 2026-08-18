#!/usr/bin/env python3
"""Не пускает Bash туда, куда нельзя писать.

Бэкенд и старая админка — спецификация поведения, а не рабочий код
(CLAUDE.md). `Edit` и `Write` по ним запрещены в settings.json, но
запрет ловит только инструменты правки: `sed -i`, `git commit` или
`> файл` из Bash проходили мимо. Эта дыра и закрывается здесь.

Читается один ключ полезной нагрузки — сама команда. Отказ отдаётся
решением PreToolUse, поэтому команда не выполняется вовсе, а причина
видна и человеку, и модели.

Правило простое: запрет срабатывает, когда в команде есть И путь
в защищённый репозиторий, И глагол, который что-то меняет. Чтение
(`grep`, `sed -n`, `cat`, `git log`, `git diff`) проходит — ради него
эти репозитории и лежат рядом.

Ложное срабатывание возможно: команда, которая лишь УПОМИНАЕТ такой
путь рядом с меняющим глаголом (например `git commit` с этим путём
в тексте сообщения), тоже будет отклонена. Цена осознанная: пропущенная
правка чужого репозитория уезжает незамеченной, а отказ виден сразу.
"""

import json
import re
import sys

# Репозитории, которые только читаются. Слэш в конце обязателен:
# иначе под запрет попадает и `ucode_backend-notes.md`.
PROTECTED = ("ucode_backend/", "ucode-frontend/")

# Глаголы, которые меняют файлы или состояние git. Читающие подкоманды
# git (status, log, diff, show, blame) сюда намеренно не попали.
WRITES = re.compile(
    r"""
      (?:^|[;&|(\s])(?:rm|mv|cp|touch|tee|mkdir|rmdir|chmod|chown|truncate|dd|patch|ln)(?:\s|$)
    | (?:^|[;&|(\s])(?:sed|perl)\s[^|;&]*-i
    | (?:^|[;&|(\s])git\b[^;&|]*?\s
      (?:commit|checkout|switch|restore|reset|clean|stash|apply|push|merge|rebase|cherry-pick|revert|add|rm|mv|tag)
      (?:\s|$)
    | >>?\s*\S*ucode[-_](?:backend|frontend)
    """,
    re.VERBOSE,
)

REASON = (
    "Это репозиторий только для чтения (CLAUDE.md: «Бэкенд — только чтение»). "
    "Правка бэкенда или старой админки — отдельная задача в отдельном репозитории "
    "с отдельным ревью. Читать (grep, sed -n, cat, git log, git diff) можно."
)


def main() -> None:
    try:
        payload = json.load(sys.stdin)
    except (json.JSONDecodeError, ValueError):
        # Полезную нагрузку не разобрали — молча пропускаем: хук,
        # который ломает работу из-за своей же ошибки, хуже отсутствующего.
        return

    command = (payload.get("tool_input") or {}).get("command") or ""

    if not any(path in command for path in PROTECTED):
        return
    if not WRITES.search(command):
        return

    json.dump(
        {
            "hookSpecificOutput": {
                "hookEventName": "PreToolUse",
                "permissionDecision": "deny",
                "permissionDecisionReason": REASON,
            }
        },
        sys.stdout,
        ensure_ascii=False,
    )


if __name__ == "__main__":
    main()

import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/shared/ui/button";
import { Field, Select } from "@/shared/ui/input";
import type { Connection, ConnectionSelection } from "../model/types";
import { AuthCard } from "./AuthCard";
import { ErrorText } from "./ErrorText";

/** Второй шаг входа: выбрать запись в каждой connection. */
export function ConnectionPicker({
  connections,
  busy,
  error,
  onSubmit,
}: {
  connections: Connection[];
  busy: boolean;
  error: Error | null;
  onSubmit: (selection: ConnectionSelection) => void;
}) {
  const { t } = useTranslation();

  // Единственная опция выбрана заранее — выбирать не из чего.
  const [selection, setSelection] = useState<ConnectionSelection>(() =>
    Object.fromEntries(
      connections.filter((c) => c.options.length === 1).map((c) => [c.id, c.options[0]!.id]),
    ),
  );

  const complete = connections.every((connection) => selection[connection.id]);

  return (
    <AuthCard
      title={t("auth.chooseWorkspace")}
      subtitle={t("auth.chooseWorkspaceSubtitle")}
      footer={null}
    >
      <form
        className="flex flex-col gap-4"
        onSubmit={(event) => {
          event.preventDefault();
          onSubmit(selection);
        }}
      >
        {connections.map((connection) => (
          <Field key={connection.id} label={connection.tableSlug}>
            <Select
              required
              value={selection[connection.id] ?? ""}
              onChange={(e) =>
                setSelection((current) => ({ ...current, [connection.id]: e.target.value }))
              }
            >
              <option value="" disabled>
                —
              </option>
              {connection.options.map((option) => (
                <option key={option.id} value={option.id}>
                  {option.label}
                </option>
              ))}
            </Select>
          </Field>
        ))}

        {error && <ErrorText error={error} />}

        <Button type="submit" disabled={busy || !complete} className="w-full">
          {t("auth.continue")}
        </Button>
      </form>
    </AuthCard>
  );
}

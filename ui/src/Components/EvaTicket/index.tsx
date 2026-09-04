import { FC, useMemo, useState } from "react";

import { observer } from "mobx-react-lite";

import { FontAwesomeIcon } from "@fortawesome/react-fontawesome";
import { faTicketAlt } from "@fortawesome/free-solid-svg-icons/faTicketAlt";
import { faSpinner } from "@fortawesome/free-solid-svg-icons/faSpinner";

import type { APIAlertGroupT, ReadOnly } from "Models/APITypes";
import { FormatBackendURI, type AlertStore } from "Stores/AlertStore";
import { Modal } from "Components/Modal";

interface EvaTicketModalProps {
  group: ReadOnly<APIAlertGroupT>;
  alertStore: AlertStore;
  isOpen: boolean;
  toggleOpen: () => void;
}

interface CreateResponseT {
  created: boolean;
  error?: string;
  ticket: {
    code: string;
    id: string;
    url: string;
    projectCode: string;
    status: string;
    identityKey: string;
    groupId: string;
  };
}

const resolveSuggestedTarget = (
  alertStore: AlertStore,
  group: ReadOnly<APIAlertGroupT>,
): string => {
  const eva = alertStore.settings.values.eva;
  for (const route of eva.routes) {
    const match = group.labels.find(
      (l) => l.name === route.label && l.value === route.value,
    );
    if (match) {
      return route.target;
    }
  }
  return eva.defaultTarget;
};

const EvaTicketModal: FC<EvaTicketModalProps> = observer(
  ({ group, alertStore, isOpen, toggleOpen }) => {
    const eva = alertStore.settings.values.eva;
    const suggested = useMemo(
      () => resolveSuggestedTarget(alertStore, group),
      [alertStore, group],
    );
    const [target, setTarget] = useState<string>(suggested);
    const [inProgress, setInProgress] = useState(false);
    const [error, setError] = useState<string | null>(null);
    const [ticket, setTicket] = useState<CreateResponseT["ticket"] | null>(
      null,
    );
    const [conflict, setConflict] = useState(false);

    const submit = async (force: boolean) => {
      setInProgress(true);
      setError(null);
      setConflict(false);
      try {
        const res = await fetch(FormatBackendURI("eva/tasks.json"), {
          method: "POST",
          credentials: "include",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            groupId: group.id,
            target,
            force,
          }),
        });
        const text = await res.text();
        let body: CreateResponseT | null = null;
        try {
          body = JSON.parse(text) as CreateResponseT;
        } catch {
          body = null;
        }
        if (res.status === 409 && body?.ticket) {
          setConflict(true);
          setTicket(body.ticket);
          setError(body.error || "Open ticket already exists");
          return;
        }
        if (!res.ok) {
          setError(body?.error || text || `HTTP ${res.status}`);
          return;
        }
        if (body?.ticket) {
          setTicket(body.ticket);
          setConflict(false);
        }
      } catch (e) {
        setError(e instanceof Error ? e.message : String(e));
      } finally {
        setInProgress(false);
      }
    };

    if (!eva.enabled) {
      return null;
    }

    return (
      <Modal isOpen={isOpen} toggleOpen={toggleOpen} size="modal-lg">
        <div className="modal-header">
          <h5 className="modal-title">
            <FontAwesomeIcon icon={faTicketAlt} className="me-2" />
            Create EVA ticket
          </h5>
          <button
            type="button"
            className="btn-close"
            onClick={toggleOpen}
            aria-label="Close"
          />
        </div>
        <div className="modal-body">
          <div className="mb-3">
            <label className="form-label">Target</label>
            <select
              className="form-select"
              value={target}
              disabled={inProgress || (!!ticket && !conflict)}
              onChange={(e) => setTarget(e.target.value)}
            >
              {eva.targets.map((t) => (
                <option key={t.code} value={t.code}>
                  {t.label} ({t.kind})
                </option>
              ))}
            </select>
          </div>
          {error ? <div className="alert alert-warning">{error}</div> : null}
          {ticket ? (
            <div className="alert alert-success">
              Ticket{" "}
              {ticket.url ? (
                <a href={ticket.url} target="_blank" rel="noopener noreferrer">
                  {ticket.code}
                </a>
              ) : (
                ticket.code
              )}{" "}
              · {ticket.status}
            </div>
          ) : null}
        </div>
        <div className="modal-footer">
          <button
            type="button"
            className="btn btn-secondary"
            onClick={toggleOpen}
          >
            Close
          </button>
          {conflict ? (
            <button
              type="button"
              className="btn btn-warning"
              disabled={inProgress}
              onClick={() => void submit(true)}
            >
              {inProgress ? (
                <FontAwesomeIcon icon={faSpinner} spin className="me-1" />
              ) : null}
              Force create
            </button>
          ) : (
            <button
              type="button"
              className="btn btn-primary"
              disabled={inProgress || !!ticket}
              onClick={() => void submit(false)}
            >
              {inProgress ? (
                <FontAwesomeIcon icon={faSpinner} spin className="me-1" />
              ) : null}
              Create
            </button>
          )}
        </div>
      </Modal>
    );
  },
);

export { EvaTicketModal, resolveSuggestedTarget };

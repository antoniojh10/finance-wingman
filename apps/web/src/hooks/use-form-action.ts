"use client";

import { useActionState, useEffect, useRef, useTransition, type FormEvent } from "react";

import { initialFormState, type FormState } from "@/lib/forms";

/**
 * Runs a Server Action returning FormState from a form's onSubmit.
 *
 * Passing the action to <form action> would make React reset the form after
 * every submission, wiping the user's input on validation errors and leaving
 * controlled <select>s out of sync with their state. Dispatching from onSubmit
 * inside a transition keeps the form as it is. onSuccess runs once per
 * successful submission, receiving the resulting state.
 */
export function useFormAction(
  action: (state: FormState, formData: FormData) => Promise<FormState>,
  onSuccess?: (state: FormState) => void,
) {
  const [state, dispatch, pending] = useActionState(action, initialFormState);
  const [, startTransition] = useTransition();
  const handled = useRef<number | undefined>(undefined);
  const callback = useRef(onSuccess);

  useEffect(() => {
    callback.current = onSuccess;
  }, [onSuccess]);

  useEffect(() => {
    if (state.ok && state.nonce !== handled.current) {
      handled.current = state.nonce;
      callback.current?.(state);
    }
  }, [state]);

  function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const data = new FormData(event.currentTarget);
    startTransition(() => dispatch(data));
  }

  return { state, onSubmit, pending };
}

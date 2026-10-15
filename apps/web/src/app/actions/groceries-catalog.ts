"use server";

import { revalidatePath } from "next/cache";
import { getTranslations } from "next-intl/server";

import { failure, success } from "@/lib/action-errors";
import { ApiError, unwrap } from "@/lib/api/client";
import { text, type FormState } from "@/lib/forms";
import { buildName, buildPriceCheck, buildProduct, buildVariant, type Built, type FieldError } from "@/lib/groceries-products";
import { authedApi } from "@/lib/session";

// Catalog mutations of the groceries products tab: products, variants, price
// checks, stores and groups.

async function invalid(errors: Record<string, FieldError>): Promise<FormState> {
  const t = await getTranslations("errors");
  const tp = await getTranslations("groceries.products.errors");
  const messages: Record<FieldError, string> = {
    required: t("required"),
    invalidNumber: t("invalidPositiveNumber"),
    invalidAmount: t("invalidPositiveNumber"),
    invalidCurrency: tp("invalidCurrency"),
  };
  return {
    ok: false,
    message: t("validation"),
    fieldErrors: Object.fromEntries(Object.entries(errors).map(([field, error]) => [field, messages[error]])),
  };
}

function done(): FormState {
  revalidatePath("/groceries", "layout");
  return success();
}

async function run<T>(built: Built<T>, call: (body: T) => Promise<unknown>, options: { conflict?: string } = {}): Promise<FormState> {
  if ("errors" in built) {
    return invalid(built.errors);
  }
  try {
    await call(built.body);
  } catch (error) {
    return failure(error, options);
  }
  return done();
}

async function conflictMessage(key: "product" | "variant" | "store" | "group"): Promise<string> {
  const t = await getTranslations("groceries.products.errors");
  return t(`${key}Exists`);
}

/** Creates a product. */
export async function createProduct(_: FormState, formData: FormData): Promise<FormState> {
  return run(
    buildProduct(formData),
    async ({ group_id, ...body }) => {
      const api = await authedApi();
      unwrap(await api.POST("/api/v1/groceries/products", { body: { ...body, group_id: group_id || undefined } }));
    },
    { conflict: await conflictMessage("product") },
  );
}

/** Edits a product; an empty group clears it. */
export async function updateProduct(_: FormState, formData: FormData): Promise<FormState> {
  const id = text(formData, "id");
  return run(
    buildProduct(formData),
    async (body) => {
      const api = await authedApi();
      unwrap(await api.PATCH("/api/v1/groceries/products/{id}", { params: { path: { id } }, body }));
    },
    { conflict: await conflictMessage("product") },
  );
}

/** Archives or restores a product. */
export async function setProductArchived(id: string, archived: boolean): Promise<FormState> {
  try {
    const api = await authedApi();
    unwrap(await api.PATCH("/api/v1/groceries/products/{id}", { params: { path: { id } }, body: { archived } }));
  } catch (error) {
    return failure(error);
  }
  return done();
}

/** Adds a variant (brand and pack) to the product in `product_id`. */
export async function createVariant(_: FormState, formData: FormData): Promise<FormState> {
  const productId = text(formData, "product_id");
  return run(
    buildVariant(formData),
    async ({ brand, pack_label, pack_size, store_id }) => {
      const api = await authedApi();
      unwrap(
        await api.POST("/api/v1/groceries/products/{id}/variants", {
          params: { path: { id: productId } },
          body: { brand, pack_label: pack_label || undefined, pack_size, store_id: store_id || undefined },
        }),
      );
    },
    { conflict: await conflictMessage("variant") },
  );
}

/** Edits a variant; empty pack size makes it loose, empty store makes it a manufacturer brand. */
export async function updateVariant(_: FormState, formData: FormData): Promise<FormState> {
  const id = text(formData, "id");
  return run(
    buildVariant(formData),
    async (body) => {
      const api = await authedApi();
      unwrap(await api.PATCH("/api/v1/groceries/variants/{id}", { params: { path: { id } }, body }));
    },
    { conflict: await conflictMessage("variant") },
  );
}

/** Deletes a variant. Receipt lines keep their text and become unassigned. */
export async function deleteVariant(id: string): Promise<FormState> {
  const api = await authedApi();
  const result = await api.DELETE("/api/v1/groceries/variants/{id}", { params: { path: { id } } });
  if (!result.response.ok) {
    return failure(new ApiError(result.response.status, result.error));
  }
  return done();
}

/** Records a price seen at a store without buying. */
export async function createPriceCheck(_: FormState, formData: FormData): Promise<FormState> {
  return run(buildPriceCheck(formData), async (body) => {
    const api = await authedApi();
    unwrap(await api.POST("/api/v1/groceries/price-checks", { body }));
  });
}

/** Creates a store (a chain). */
export async function createStore(_: FormState, formData: FormData): Promise<FormState> {
  return run(
    buildName(formData),
    async ({ name }) => {
      const api = await authedApi();
      unwrap(await api.POST("/api/v1/groceries/stores", { body: { name } }));
    },
    { conflict: await conflictMessage("store") },
  );
}

/** Renames a store. */
export async function renameStore(_: FormState, formData: FormData): Promise<FormState> {
  const id = text(formData, "id");
  return run(
    buildName(formData),
    async ({ name }) => {
      const api = await authedApi();
      unwrap(await api.PATCH("/api/v1/groceries/stores/{id}", { params: { path: { id } }, body: { name } }));
    },
    { conflict: await conflictMessage("store") },
  );
}

/** Archives or restores a store. */
export async function setStoreArchived(id: string, archived: boolean): Promise<FormState> {
  try {
    const api = await authedApi();
    unwrap(await api.PATCH("/api/v1/groceries/stores/{id}", { params: { path: { id } }, body: { archived } }));
  } catch (error) {
    return failure(error, { conflict: await conflictMessage("store") });
  }
  return done();
}

/** Creates a product group. */
export async function createGroup(_: FormState, formData: FormData): Promise<FormState> {
  return run(
    buildName(formData),
    async ({ name, is_food }) => {
      const api = await authedApi();
      unwrap(await api.POST("/api/v1/groceries/groups", { body: { name, is_food } }));
    },
    { conflict: await conflictMessage("group") },
  );
}

/** Renames a product group. */
export async function renameGroup(_: FormState, formData: FormData): Promise<FormState> {
  const id = text(formData, "id");
  return run(
    buildName(formData),
    async ({ name }) => {
      const api = await authedApi();
      unwrap(await api.PATCH("/api/v1/groceries/groups/{id}", { params: { path: { id } }, body: { name } }));
    },
    { conflict: await conflictMessage("group") },
  );
}

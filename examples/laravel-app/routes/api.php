<?php

use Illuminate\Http\Request;
use Illuminate\Http\JsonResponse;
use Illuminate\Support\Facades\Route;
use Illuminate\Support\Str;

/*
|--------------------------------------------------------------------------
| API Routes
|--------------------------------------------------------------------------
|
| NexusOps example Laravel API routes. These routes are loaded by the
| RouteServiceProvider and are assigned the "api" middleware group.
|
*/

// ---------------------------------------------------------------------------
// Health Check
// ---------------------------------------------------------------------------

Route::get('/health', function (): JsonResponse {
    return response()->json([
        'status'  => 'healthy',
        'service' => 'laravel-api',
        'php'     => PHP_VERSION,
        'laravel' => app()->version(),
    ]);
});

// ---------------------------------------------------------------------------
// Products Resource
// ---------------------------------------------------------------------------

Route::prefix('products')->group(function () {

    // In-memory store for demo purposes — a real app would use Eloquent models.
    $products = collect();

    // List all products.
    Route::get('/', function () use (&$products): JsonResponse {
        return response()->json($products->values());
    });

    // Get a single product.
    Route::get('/{id}', function (string $id) use (&$products): JsonResponse {
        $product = $products->firstWhere('id', $id);

        if (!$product) {
            return response()->json(['error' => 'Product not found'], 404);
        }

        return response()->json($product);
    });

    // Create a product.
    Route::post('/', function (Request $request) use (&$products): JsonResponse {
        $validated = $request->validate([
            'name'  => 'required|string|max:255',
            'price' => 'required|numeric|min:0',
            'sku'   => 'nullable|string|max:64',
        ]);

        $product = [
            'id'         => (string) Str::uuid(),
            'name'       => $validated['name'],
            'price'      => (float) $validated['price'],
            'sku'        => $validated['sku'] ?? null,
            'created_at' => now()->toIso8601String(),
            'updated_at' => now()->toIso8601String(),
        ];

        $products->push($product);

        return response()->json($product, 201);
    });

    // Update a product.
    Route::put('/{id}', function (Request $request, string $id) use (&$products): JsonResponse {
        $index = $products->search(fn ($p) => $p['id'] === $id);

        if ($index === false) {
            return response()->json(['error' => 'Product not found'], 404);
        }

        $validated = $request->validate([
            'name'  => 'sometimes|required|string|max:255',
            'price' => 'sometimes|required|numeric|min:0',
            'sku'   => 'nullable|string|max:64',
        ]);

        $existing = $products->get($index);
        $updated  = array_merge($existing, $validated, [
            'updated_at' => now()->toIso8601String(),
        ]);

        $products->put($index, $updated);

        return response()->json($updated);
    });

    // Delete a product.
    Route::delete('/{id}', function (string $id) use (&$products): JsonResponse {
        $index = $products->search(fn ($p) => $p['id'] === $id);

        if ($index === false) {
            return response()->json(['error' => 'Product not found'], 404);
        }

        $products->forget($index);

        return response()->json(null, 204);
    });
});

// ---------------------------------------------------------------------------
// Authenticated user info (Sanctum example)
// ---------------------------------------------------------------------------

Route::middleware('auth:sanctum')->get('/user', function (Request $request): JsonResponse {
    return response()->json($request->user());
});

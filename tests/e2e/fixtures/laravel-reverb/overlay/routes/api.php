<?php

use Illuminate\Support\Facades\DB;
use Illuminate\Support\Facades\Route;
use App\Jobs\TestBroadcastJob;

Route::get('/health', function () {
    return response()->json(['status' => 'ok']);
});

Route::post('/setup', function () {
    try {
        Artisan::call('migrate', ['--force' => true]);
    } catch (\Throwable $e) {
        // If tables already exist from a partial previous run, that's OK.
        if (!str_contains($e->getMessage(), 'already exists')) {
            throw $e;
        }
    }
    return response()->json(['status' => 'migrated']);
});

Route::post('/dispatch-test', function () {
    $marker = request()->input('marker', 'default');
    TestBroadcastJob::dispatch($marker);
    return response()->json(['status' => 'dispatched', 'marker' => $marker]);
});

Route::get('/check-result', function () {
    $result = DB::table('test_results')->where('key', 'marker')->first();
    if (!$result) {
        return response()->json(['status' => 'pending'], 202);
    }
    return response()->json([
        'status' => 'completed',
        'marker' => $result->value,
    ]);
});

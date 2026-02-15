<?php

use Illuminate\Support\Facades\Route;
use App\Jobs\TestBroadcastJob;

Route::get('/health', function () {
    return response()->json(['status' => 'ok']);
});

Route::post('/setup', function () {
    Artisan::call('migrate', ['--force' => true]);
    return response()->json(['status' => 'migrated']);
});

Route::post('/dispatch-test', function () {
    $marker = request()->input('marker', 'default');
    TestBroadcastJob::dispatch($marker);
    return response()->json(['status' => 'dispatched', 'marker' => $marker]);
});

Route::get('/check-result', function () {
    $path = storage_path('app/test-broadcast-marker.txt');
    if (!file_exists($path)) {
        return response()->json(['status' => 'pending'], 202);
    }
    return response()->json([
        'status' => 'completed',
        'marker' => trim(file_get_contents($path)),
    ]);
});

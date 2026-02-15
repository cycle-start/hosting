<?php

namespace App\Jobs;

use App\Events\TestBroadcastEvent;
use Illuminate\Bus\Queueable;
use Illuminate\Contracts\Queue\ShouldQueue;
use Illuminate\Foundation\Bus\Dispatchable;
use Illuminate\Queue\InteractsWithQueue;
use Illuminate\Queue\SerializesModels;
use Illuminate\Support\Facades\DB;

class TestBroadcastJob implements ShouldQueue
{
    use Dispatchable, InteractsWithQueue, Queueable, SerializesModels;

    public function __construct(
        public string $marker,
    ) {}

    public function handle(): void
    {
        // Write marker to database so the check-result endpoint can verify
        // from any node (filesystem isn't shared in dev without CephFS).
        DB::table('test_results')->updateOrInsert(
            ['key' => 'marker'],
            ['value' => $this->marker, 'updated_at' => now()],
        );

        // Broadcast the event via Reverb.
        event(new TestBroadcastEvent($this->marker));
    }
}

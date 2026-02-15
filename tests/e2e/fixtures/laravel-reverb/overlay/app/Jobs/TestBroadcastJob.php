<?php

namespace App\Jobs;

use App\Events\TestBroadcastEvent;
use Illuminate\Bus\Queueable;
use Illuminate\Contracts\Queue\ShouldQueue;
use Illuminate\Foundation\Bus\Dispatchable;
use Illuminate\Queue\InteractsWithQueue;
use Illuminate\Queue\SerializesModels;

class TestBroadcastJob implements ShouldQueue
{
    use Dispatchable, InteractsWithQueue, Queueable, SerializesModels;

    public function __construct(
        public string $marker,
    ) {}

    public function handle(): void
    {
        // Write marker to file so the check-result endpoint can verify.
        file_put_contents(
            storage_path('app/test-broadcast-marker.txt'),
            $this->marker,
        );

        // Broadcast the event via Reverb.
        event(new TestBroadcastEvent($this->marker));
    }
}

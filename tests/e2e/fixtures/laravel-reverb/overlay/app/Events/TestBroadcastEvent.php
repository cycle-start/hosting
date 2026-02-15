<?php

namespace App\Events;

use Illuminate\Broadcasting\Channel;
use Illuminate\Contracts\Broadcasting\ShouldBroadcast;
use Illuminate\Queue\SerializesModels;

class TestBroadcastEvent implements ShouldBroadcast
{
    use SerializesModels;

    public function __construct(
        public string $marker,
    ) {}

    public function broadcastOn(): Channel
    {
        return new Channel('test-channel');
    }

    public function broadcastAs(): string
    {
        return 'test.broadcast';
    }
}

<?php
declare(strict_types=1);

// Minimal PHP example app
// This file will be modified by the injector to include otel.php when running codegen

// If composer autoload exists (after codegen adds dependencies), require it to load OTEL SDK
if (file_exists(__DIR__ . '/vendor/autoload.php')) {
    require __DIR__ . '/vendor/autoload.php';
}

function main(): void {
    echo "Hello, PHP!\n";
}

main();



<?php
declare(strict_types=1);

use Slim\Factory\AppFactory;
use Psr\Http\Message\ResponseInterface as Response;
use Psr\Http\Message\ServerRequestInterface as Request;

// Create Slim app
$app = AppFactory::create();

// Add error middleware
$app->addErrorMiddleware(true, true, true);

// Define routes
$app->get('/', function (Request $request, Response $response) {
    $response->getBody()->write('Hello, PHP!');
    return $response;
});

$app->get('/health', function (Request $request, Response $response) {
    $response->getBody()->write('OK');
    return $response;
});

// Run the app
$app->run();
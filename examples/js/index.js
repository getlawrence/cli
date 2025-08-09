const express = require('express');

const app = express();
const port = 3000;

app.get('/', (req, res) => {
    res.send('Hello from Service C!');
});

app.listen(port, () => {
    console.log(`Service C is running at http://localhost:${port}`);
});
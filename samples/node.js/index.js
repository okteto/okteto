const express = require('express');
const app = express();

const PORT = 3000;

app.get('/', function (req, res) {
  res.send('Hello world!');
});

app.listen(PORT, function () {
  console.log('Started hello-world server on %d', PORT);
});

const webpack = require('webpack');
const merge = require('webpack-merge');
const baseConfig = require('./webpack.config.js');

module.exports = merge({
  mode: 'production',
  plugins: [
    new webpack.DefinePlugin({
      MODE: JSON.stringify('production')
    })
  ]
}, baseConfig);

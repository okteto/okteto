const path = require('path');
const CopyWebpackPlugin = require('copy-webpack-plugin');
const HtmlWebpackPlugin = require('html-webpack-plugin');

const appPath = path.join(__dirname, '/src')

module.exports = {
  context: appPath,
  entry: [
    'whatwg-fetch',
    './index.jsx'
  ],
  output: {
    filename: 'app.[hash].js',
    path: '/pod-data',
  },
  resolve: {
    extensions: ['.js', '.jsx', '.scss'],
    modules: [
      path.resolve(path.join(__dirname, '/node_modules')),
      path.resolve(appPath)
    ]
  },
  module: {
    rules: [{
      test: /\.jsx?$/,
      exclude: /node_modules/,
      loaders: ['babel-loader'],
    }, {
      test: /\.js?$/,
      exclude: /node_modules/,
      loaders: ['babel-loader'],
    }, {
      test: /\.css$/,
      use: [{
        loader: 'style-loader'
      }, {
        loader: 'css-loader',
        options: {
          includePaths: [appPath]
        }
      }, {
        loader: 'postcss-loader'
      }]
    }, 
    {
      test: /\.(scss|sass)$/,
      use: [{
        loader: 'style-loader'
      }, {
        loader: 'css-loader'
      }, {
        loader: 'postcss-loader'
      }, {
        loader: 'fast-sass-loader',
        options: {
          includePaths: [appPath]
        }
      }]
    }, {
      test: /\.html$/,
      loader: 'file-loader?name=[name].[ext]',
    }, { 
      test: /\.(png|woff|woff2|eot|ttf|svg)$/, 
      loader: 'url-loader?limit=100000' 
    }, { 
      test: /\.hbs$/,
      loader: 'handlebars-loader'
    }],
  },
  plugins: [
    new CopyWebpackPlugin([
      { from: 'favicon*' }
    ]),
    new HtmlWebpackPlugin({
      template: './index.hbs'
    })
  ]
};
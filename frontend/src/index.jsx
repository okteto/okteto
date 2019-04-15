import React from 'react';
import ReactDOM from 'react-dom';
import { createStore, applyMiddleware, compose } from 'redux';
import { Provider } from 'react-redux';
import thunk from 'redux-thunk';
import smoothscroll from 'smoothscroll-polyfill';
import 'whatwg-fetch';

import reducers from 'reducers';
import AppView from 'views/AppView';

import 'index.scss';

// To enable Redux devtools.
const composeEnhancers = window.__REDUX_DEVTOOLS_EXTENSION_COMPOSE__ || compose;
const store = createStore(reducers, composeEnhancers(applyMiddleware(thunk)));
smoothscroll.polyfill();

ReactDOM.render(
  <Provider store={ store }>
    <AppView />
  </Provider>,
  document.getElementById('app')
);


import React, { Component } from 'react';
import * as ReactRedux from 'react-redux';
import PropTypes from 'prop-types';
import { hot } from 'react-hot-loader';

import Space from 'containers/Space';
import Login from 'containers/Login';
import Notification from 'components/Notification';

import { restoreSession } from 'actions/session';

import 'views/AppView.scss';

class AppView extends Component {
  constructor(props) {
    super(props);

    // Restore any existing session.
    this.props.dispatch(restoreSession());
  }

  render() {
    return (
      <div className="AppView">
        <Notification />
        {!this.props.session.isAuthenticated &&
          <Login />
        }
        {this.props.session.isAuthenticated &&
          <Space />
        }
      </div>
    );
  }
}

AppView.propTypes = {
  session: PropTypes.object.isRequired,
  dispatch: PropTypes.func
};

const AppViewWithRedux = ReactRedux.connect(state => {
  return {
    session: state.session
  };
})(AppView);

// Enable React Hot Loader for the root component.
export default hot(module)(AppViewWithRedux);
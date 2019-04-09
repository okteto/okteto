import React, { Component } from 'react';
import * as ReactRedux from 'react-redux';
import PropTypes from 'prop-types';
import { hot } from 'react-hot-loader';

import MainContainer from 'containers/MainContainer';
import LoginContainer from 'containers/LoginContainer';
import Notification from 'components/Notification';

import 'views/AppView.scss';

class AppView extends Component {
  constructor(props) {
    super(props);
  }

  render() {
    return (
      <div className="AppView">
        <Notification />
        {!this.props.session.isAuthenticated &&
          <LoginContainer />
        }
        {this.props.session.isAuthenticated &&
          <MainContainer />
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
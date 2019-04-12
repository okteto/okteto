import React, { Component } from 'react';
import * as ReactRedux from 'react-redux';
import PropTypes from 'prop-types';
import colors from 'colors.scss';

import UserMenu from 'components/UserMenu';
import Icon from 'components/Icon';
import { logout } from 'actions/session';

import 'containers/Header.scss';

class Header extends Component {
  constructor(props) {
    super(props);

    this.state = {
      showUserMenu: false
    };
  }

  render() {
    const { user } = this.props;
    return (
      <div className="Header horizontal layout center">
        {`${user.id}'s space`}
        <div className="flex-auto" />
        <div className="User">
          <div className="UserAtom layout horizontal center"
            onClick={() => this.setState({ showUserMenu: true })}>
            <div className="Avatar">
              <Icon icon="logo" size="52" color={colors.navyDark} />
            </div>
            <div className="Username">{user.id}</div>
            <Icon icon="arrowDown" size="24" color="white" />
          </div>
          {this.state.showUserMenu && 
            <UserMenu
              user={user} 
              onLogout={() => {
                this.props.dispatch(logout());
              }}
              onClose={() => this.setState({ showUserMenu: false })}
            />
          }
        </div>
      </div>
    );  
  }
}

Header.propTypes = {
  dispatch: PropTypes.func,
  user: PropTypes.object.isRequired
};

export default ReactRedux.connect(state => {
  return {
    user: state.session.user
  };
})(Header);
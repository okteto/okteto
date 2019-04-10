import React, { Component } from 'react';
import PropTypes from 'prop-types';
import autobind from 'autobind-decorator';

import Icon from 'components/Icon';

import 'components/UserMenu.scss';

class UserMenu extends Component {
  constructor(props) {
    super(props);
  }

  componentDidMount() {
    document.addEventListener('click', this.handleOutsideClick);
  }
  componentWillUnmount() {
    document.removeEventListener('click', this.handleOutsideClick);
  }

  @autobind
  handleOutsideClick() {
    this.props.onClose && this.props.onClose();
  }
  
  render() {
    const { user } = this.props;
    return (
      <div className="UserMenu MenuItemList layout vertical" 
        onClick={this.handleOutsideClick}>
        <div className="MenuTitle layout horizontal center flex-auto">
          {user.username}
        </div>
        <div className="MenuItem layout horizontal center flex-auto">
          <Icon icon="plus" size="22" color="black" />
          Profile
        </div>
        <div className="MenuItem layout horizontal center flex-auto"
          onClick={this.props.onLogout}>
          <Icon icon="plus" size="22" color="black" />
          Log out
        </div>
      </div>
    );
  }
}

UserMenu.propTypes = {
  user: PropTypes.object.isRequired,
  onLogout: PropTypes.func.isRequired,
  onClose: PropTypes.func
};

export default UserMenu;
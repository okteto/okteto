import React, { Component } from 'react';
import PropTypes from 'prop-types';
import autobind from 'autobind-decorator';

import Icon from 'components/Icon';

import 'components/NewMenu.scss';

class NewMenu extends Component {
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
    return (
      <div className="NewMenu MenuItemList layout vertical" 
        onClick={this.handleOutsideClick}>
        <div className="MenuItem MenuItemEnvironment layout horizontal center flex-auto"
          onClick={this.props.onNewEnvironment}>
          <Icon icon="mirror" size="18" color="black" />
          Environment
        </div>
        <div className="MenuItem MenuItemDatabase layout horizontal center flex-auto"
          onClick={this.props.onNewDatabase}>
          <Icon icon="database" size="22" color="black" />
          Database
        </div>
      </div>
    );
  }
}

NewMenu.propTypes = {
  onNewEnvironment: PropTypes.func.isRequired,
  onNewDatabase: PropTypes.func.isRequired,
  onClose: PropTypes.func
};

export default NewMenu;
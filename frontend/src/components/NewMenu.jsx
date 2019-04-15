import React, { Component } from 'react';
import PropTypes from 'prop-types';
import autobind from 'autobind-decorator';

import Icon from 'components/Icon';

import 'components/NewMenu.scss';

class NewMenu extends Component {
  constructor(props) {
    super(props);
  }

  static NewMenuContent(onNewEnvironment, onNewDatabase) {
    return () => (
      <div className="NewMenuContent layout vertical">
        <div className="MenuItem MenuItemEnvironment layout horizontal center flex-auto"
          onClick={onNewEnvironment}>
          <Icon icon="mirror" size="18" color="black" />
          Environment
        </div>
        <div className="MenuItem MenuItemDatabase layout horizontal center flex-auto"
          onClick={onNewDatabase}>
          <Icon icon="database" size="22" color="black" />
          Database
        </div>
      </div>
    );
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
    const NewMenuContent = NewMenu.NewMenuContent(
      this.props.onNewEnvironment, this.props.onNewDatabase);

    return (
      <div className={`NewMenu MenuItemList layout vertical position-${this.props.position}`}
        onClick={this.handleOutsideClick}>
        <NewMenuContent />
      </div>
    );
  }
}

NewMenu.defaultProps = {
  position: 'left'
};

NewMenu.propTypes = {
  position: PropTypes.string,
  onNewEnvironment: PropTypes.func.isRequired,
  onNewDatabase: PropTypes.func.isRequired,
  onClose: PropTypes.func
};

export default NewMenu;